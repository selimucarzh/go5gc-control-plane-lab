const form = document.querySelector("#flowForm");
const timeline = document.querySelector("#timeline");
const amfStatus = document.querySelector("#amfStatus");
const smfStatus = document.querySelector("#smfStatus");
const ueState = document.querySelector("#ueState");
const sessionState = document.querySelector("#sessionState");
const eventState = document.querySelector("#eventState");

document.querySelector("#registerBtn").addEventListener("click", () => registerUE());
document.querySelector("#authBtn").addEventListener("click", () => authenticateUE());
document.querySelector("#pduBtn").addEventListener("click", () => createPDUSession());
document.querySelector("#fullFlowBtn").addEventListener("click", runFullFlow);
document.querySelector("#resetBtn").addEventListener("click", resetState);
document.querySelector("#refreshStateBtn").addEventListener("click", refreshState);

checkHealth();
refreshState();

async function checkHealth() {
  const [amf, smf] = await Promise.allSettled([
    fetchJSON("/api/amf/healthz"),
    fetchJSON("/api/smf/healthz"),
  ]);

  setStatus(amfStatus, amf.status === "fulfilled" && amf.value.status < 300);
  setStatus(smfStatus, smf.status === "fulfilled" && smf.value.status < 300);
}

async function runFullFlow() {
  clearTimeline();
  await registerUE();
  await authenticateUE();
  await createPDUSession();
}

async function registerUE() {
  const values = readForm();
  const body = {
    protocol_discriminator: "5GMM",
    security_header_type: "plain_5gs_nas_message",
    message_type: "RegistrationRequest",
    sequence_number: 1,
    payload: {
      supi: values.supi,
      plmn_id: values.plmn,
      access_type: values.access,
      registration_type: "initial_registration",
      ngksi: 1,
      requested_nssai: [
        {
          sst: Number(values.sst),
          sd: values.sd,
        },
      ],
      ue_security_capability: ["nea2", "nia2"],
    },
  };

  const result = await fetchJSON("/api/amf/nas", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  addTrace("UE -> AMF NAS registration", "POST /nas", body, result);
  await checkHealth();
  await refreshState();
  return result;
}

async function authenticateUE() {
  const values = readForm();
  const challenge = await fetchJSON(`/api/amf/ues/${encodeURIComponent(values.supi)}/authentication`, {
    method: "POST",
  });

  addTrace("AMF -> UE authentication challenge", "POST /ues/{supi}/authentication", { supi: values.supi }, challenge);
  if (!challenge.ok || !challenge.body.vector?.xres_star) {
    await refreshState();
    return challenge;
  }

  const body = {
    res_star: challenge.body.vector.xres_star,
  };
  const confirm = await fetchJSON(`/api/amf/ues/${encodeURIComponent(values.supi)}/authentication/confirm`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  addTrace("UE -> AMF authentication response", "POST /ues/{supi}/authentication/confirm", body, confirm);
  await refreshState();
  return confirm;
}

async function createPDUSession() {
  const values = readForm();
  const body = {
    supi: values.supi,
    session_id: Number(values.session),
    dnn: values.dnn,
    s_nssai: {
      sst: Number(values.sst),
      sd: values.sd,
    },
  };

  const result = await fetchJSON("/api/smf/pdu-sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  addTrace("UE -> SMF PDU session", "POST /pdu-sessions", body, result);
  await checkHealth();
  await refreshState();
  return result;
}

async function resetState() {
  const [smfReset, amfReset] = await Promise.all([
    fetchJSON("/api/smf/pdu-sessions", { method: "DELETE" }),
    fetchJSON("/api/amf/ues", { method: "DELETE" }),
  ]);

  clearTimeline();
  addTrace("Lab state reset", "DELETE /pdu-sessions + DELETE /ues", {}, {
    status: smfReset.ok && amfReset.ok ? 200 : 500,
    ok: smfReset.ok && amfReset.ok,
    body: {
      smf: smfReset.body,
      amf: amfReset.body,
    },
  });
  await checkHealth();
  await refreshState();
}

async function refreshState() {
  const [ues, sessions, events] = await Promise.allSettled([
    fetchJSON("/api/amf/ues"),
    fetchJSON("/api/smf/pdu-sessions"),
    fetchJSON("/api/amf/events"),
  ]);

  renderState(ueState, ues);
  renderState(sessionState, sessions);
  renderState(eventState, events);
}

function readForm() {
  return Object.fromEntries(new FormData(form).entries());
}

async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  const text = await response.text();
  let payload;

  try {
    payload = text ? JSON.parse(text) : {};
  } catch {
    payload = { raw: text };
  }

  return {
    status: response.status,
    ok: response.ok,
    body: payload,
  };
}

function setStatus(element, ok) {
  element.classList.toggle("ok", ok);
  element.classList.toggle("fail", !ok);
  element.classList.remove("pending");
}

function clearTimeline() {
  timeline.replaceChildren();
}

function renderState(element, result) {
  if (result.status !== "fulfilled") {
    element.textContent = JSON.stringify({ error: "request failed" }, null, 2);
    return;
  }

  element.textContent = JSON.stringify(result.value.body, null, 2);
}

function addTrace(title, endpoint, request, result) {
  const empty = timeline.querySelector(".empty");
  if (empty) {
    empty.remove();
  }

  const item = document.createElement("li");
  const badgeClass = result.ok ? "ok" : "error";
  item.innerHTML = `
    <div class="trace-head">
      <strong>${escapeHTML(title)}</strong>
      <span class="badge ${badgeClass}">HTTP ${result.status}</span>
    </div>
    <p class="trace-endpoint">${escapeHTML(endpoint)}</p>
    <pre>${escapeHTML(JSON.stringify({ request, response: result.body }, null, 2))}</pre>
  `;
  timeline.append(item);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

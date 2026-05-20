const form = document.querySelector("#flowForm");
const timeline = document.querySelector("#timeline");
const amfStatus = document.querySelector("#amfStatus");
const smfStatus = document.querySelector("#smfStatus");

document.querySelector("#registerBtn").addEventListener("click", () => registerUE());
document.querySelector("#pduBtn").addEventListener("click", () => createPDUSession());
document.querySelector("#fullFlowBtn").addEventListener("click", runFullFlow);

checkHealth();

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
  await createPDUSession();
}

async function registerUE() {
  const values = readForm();
  const body = {
    supi: values.supi,
    plmn_id: values.plmn,
    access_type: values.access,
  };

  const result = await fetchJSON("/api/amf/registration", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  addTrace("UE -> AMF registration", "POST /registration", body, result);
  await checkHealth();
  return result;
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
  return result;
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

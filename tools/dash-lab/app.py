from dash import Dash, Input, Output, State, callback, ctx, dcc, html


STAGES = [
    {
        "id": "repo-setup",
        "title": "1. Go workspace setup",
        "tag": "Foundation",
        "summary": "Created a small Go module for 5GC control-plane exercises.",
        "details": [
            "Established the repository layout with cmd and internal packages.",
            "Kept the first scope small enough to run locally without external 5G infrastructure.",
            "Made the codebase suitable for incremental AMF, SMF, and simulator work.",
        ],
        "value": "This gave us a clean learning sandbox where each control-plane concept can be added and tested separately.",
    },
    {
        "id": "cp-stub",
        "title": "2. cp-stub and UE simulator",
        "tag": "First flow",
        "summary": "Built a simple control-plane stub plus a UE simulator client.",
        "details": [
            "cp-stub exposes registration and PDU session endpoints.",
            "ue-sim can generate IMSI/SUPI identities and send requests.",
            "The stub keeps UE and PDU session state in memory.",
        ],
        "value": "This created the first end-to-end request path before splitting responsibilities into AMF and SMF.",
    },
    {
        "id": "amf",
        "title": "3. AMF registration path",
        "tag": "AMF",
        "summary": "Separated AMF registration into its own service.",
        "details": [
            "AMF accepts SUPI, PLMN ID, and access type.",
            "AMF stores UE context in memory.",
            "AMF exposes UE lookup through /ues/{supi}.",
        ],
        "value": "This models the first important control-plane responsibility: accepting and tracking registered UEs.",
    },
    {
        "id": "smf",
        "title": "4. SMF PDU session path",
        "tag": "SMF",
        "summary": "Added SMF as a separate service that verifies UE registration with AMF.",
        "details": [
            "SMF receives PDU session requests.",
            "SMF calls AMF before creating a session.",
            "Unregistered UEs are rejected before session creation.",
        ],
        "value": "This shows service-to-service control-plane coordination instead of a single combined stub.",
    },
    {
        "id": "kubernetes",
        "title": "5. Kubernetes deployment setup",
        "tag": "Kubernetes",
        "summary": "Added Docker and Kubernetes deployment assets for AMF and SMF.",
        "details": [
            "Dockerfile builds either AMF or SMF with a SERVICE build argument.",
            "deploy/k8s contains namespace, AMF, SMF, and kustomization manifests.",
            "Bootstrap notes document cluster repair, CNI, and kube-proxy steps.",
        ],
        "value": "This moves the same learning flow from local processes toward a realistic service deployment shape.",
    },
    {
        "id": "lab-viewer",
        "title": "6. Local lab viewer",
        "tag": "Visualization",
        "summary": "Added a browser UI that starts AMF, SMF, and a visual flow on localhost.",
        "details": [
            "cmd/lab-viewer starts AMF on 8081, SMF on 8082, and UI on 8090.",
            "The UI runs real registration and PDU session calls through a proxy.",
            "It records request and response payloads as a sequence trace.",
        ],
        "value": "This turns the code into an interactive training artifact instead of only terminal commands.",
    },
]

NEXT_STEPS = [
    ("Reset state", "Add endpoints to clear AMF UE contexts and SMF session contexts between demos."),
    ("State panels", "Expose /ues and /pdu-sessions list endpoints and show live in-memory state."),
    ("Failure paths", "Add guided scenarios for unregistered UE, invalid SUPI, invalid DNN, and duplicate session."),
    ("Kubernetes mode", "Point the viewer at port-forwarded Kubernetes services for the same visual flow."),
    ("Protocol depth", "Add simplified NAS/Nsmf labels so the lab maps closer to 5GC terminology."),
]

ARCHITECTURE_NODES = [
    ("UE simulator", "Generates SUPI and sends learning requests.", "ue"),
    ("AMF", "Registers UE context and serves UE lookup.", "amf"),
    ("SMF", "Creates PDU sessions after AMF verification.", "smf"),
    ("Kubernetes", "Runs AMF and SMF as deployable services.", "k8s"),
]


app = Dash(__name__)
app.title = "go5gc learning recap"


def stage_button(stage, index):
    return html.Button(
        [
            html.Span(stage["tag"], className="stage-tag"),
            html.Strong(stage["title"]),
            html.Small(stage["summary"]),
        ],
        id={"type": "stage-button", "index": index},
        className="stage-button",
        n_clicks=0,
    )


def metric(label, value):
    return html.Div(
        [html.Strong(value), html.Span(label)],
        className="metric",
    )


def layout():
    return html.Div(
        [
            html.Header(
                [
                    html.Div(
                        [
                            html.P("go5gc-control-plane-lab", className="eyebrow"),
                            html.H1("Control-plane learning recap"),
                            html.P(
                                "A Dash interface that explains what we have built, why each step exists, and where the lab can go next.",
                                className="hero-copy",
                            ),
                        ],
                        className="hero-text",
                    ),
                    html.Div(
                        [
                            metric("implemented stages", str(len(STAGES))),
                            metric("local services", "AMF + SMF"),
                            metric("viewer ports", "8081 / 8082 / 8090"),
                        ],
                        className="metrics",
                    ),
                ],
                className="hero",
            ),
            html.Main(
                [
                    html.Section(
                        [
                            html.Div(
                                [
                                    html.H2("Stage timeline"),
                                    html.P("Select a stage to inspect the work and the learning value."),
                                ],
                                className="section-heading",
                            ),
                            html.Div(
                                [stage_button(stage, index) for index, stage in enumerate(STAGES)],
                                className="stage-list",
                            ),
                        ],
                        className="timeline-panel",
                    ),
                    html.Section(
                        [
                            html.Div(id="stage-detail", className="detail-panel"),
                            html.Div(
                                [
                                    html.Div(
                                        [
                                            html.H2("Architecture map"),
                                            html.P("The current repo moves from a single stub to separated control-plane services."),
                                        ],
                                        className="section-heading",
                                    ),
                                    html.Div(
                                        [
                                            html.Div(
                                                [
                                                    html.Span(kind.upper(), className=f"node-badge {kind}"),
                                                    html.H3(name),
                                                    html.P(description),
                                                ],
                                                className="arch-node",
                                            )
                                            for name, description, kind in ARCHITECTURE_NODES
                                        ],
                                        className="arch-grid",
                                    ),
                                ],
                                className="architecture-panel",
                            ),
                            html.Div(
                                [
                                    html.Div(
                                        [
                                            html.H2("Recommended next development"),
                                            html.P("These are the next useful increments for an educational control-plane lab."),
                                        ],
                                        className="section-heading",
                                    ),
                                    html.Ul(
                                        [
                                            html.Li([html.Strong(title), html.Span(text)])
                                            for title, text in NEXT_STEPS
                                        ],
                                        className="next-list",
                                    ),
                                ],
                                className="next-panel",
                            ),
                        ],
                        className="content-grid",
                    ),
                ]
            ),
            dcc.Store(id="selected-stage", data=0),
        ],
        className="page",
    )


app.layout = layout


@callback(
    Output("selected-stage", "data"),
    Input({"type": "stage-button", "index": 0}, "n_clicks"),
    Input({"type": "stage-button", "index": 1}, "n_clicks"),
    Input({"type": "stage-button", "index": 2}, "n_clicks"),
    Input({"type": "stage-button", "index": 3}, "n_clicks"),
    Input({"type": "stage-button", "index": 4}, "n_clicks"),
    Input({"type": "stage-button", "index": 5}, "n_clicks"),
    State("selected-stage", "data"),
)
def select_stage(*values):
	current = values[-1] or 0
	if not ctx.triggered_id:
		return current
	return ctx.triggered_id["index"]


@callback(Output("stage-detail", "children"), Input("selected-stage", "data"))
def render_stage_detail(index):
    stage = STAGES[index]
    return [
        html.Div(
            [
                html.Span(stage["tag"], className="detail-tag"),
                html.H2(stage["title"]),
                html.P(stage["summary"], className="detail-summary"),
            ],
            className="detail-heading",
        ),
        html.Div(
            [
                html.H3("What we built"),
                html.Ul([html.Li(item) for item in stage["details"]]),
            ],
            className="detail-block",
        ),
        html.Div(
            [
                html.H3("Why it matters"),
                html.P(stage["value"]),
            ],
            className="detail-block emphasis",
        ),
    ]


app.index_string = """
<!doctype html>
<html>
  <head>
    {%metas%}
    <title>{%title%}</title>
    {%favicon%}
    {%css%}
    <style>
      :root {
        --bg: #f4f7f5;
        --panel: #ffffff;
        --ink: #17211d;
        --muted: #627069;
        --line: #d8e1dc;
        --green: #1d8259;
        --blue: #2468b8;
        --amber: #a76815;
        --red: #b54242;
        --shadow: 0 18px 38px rgba(23, 33, 29, 0.08);
      }

      * { box-sizing: border-box; }

      body {
        margin: 0;
        background: var(--bg);
        color: var(--ink);
        font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }

      button, input, select { font: inherit; }

      .page { min-height: 100vh; }

      .hero {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(260px, 420px);
        gap: 28px;
        align-items: end;
        padding: 34px clamp(18px, 4vw, 58px) 28px;
        border-bottom: 1px solid var(--line);
        background: #fbfcfb;
      }

      .eyebrow {
        margin: 0 0 10px;
        color: var(--green);
        font-size: 13px;
        font-weight: 800;
        text-transform: uppercase;
      }

      h1, h2, h3, p { margin-top: 0; }

      h1 {
        max-width: 780px;
        margin-bottom: 14px;
        font-size: clamp(34px, 5vw, 64px);
        line-height: 1.02;
      }

      h2 { margin-bottom: 7px; font-size: 24px; }
      h3 { margin-bottom: 9px; font-size: 17px; }

      .hero-copy {
        max-width: 760px;
        margin-bottom: 0;
        color: var(--muted);
        font-size: 18px;
        line-height: 1.55;
      }

      .metrics {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: 10px;
      }

      .metric {
        min-height: 92px;
        border: 1px solid var(--line);
        border-radius: 8px;
        padding: 14px;
        background: var(--panel);
      }

      .metric strong,
      .metric span { display: block; }

      .metric strong { margin-bottom: 8px; font-size: 23px; }
      .metric span { color: var(--muted); font-size: 13px; font-weight: 700; }

      main {
        display: grid;
        grid-template-columns: minmax(290px, 430px) minmax(0, 1fr);
        gap: 22px;
        padding: 24px clamp(18px, 4vw, 58px) 48px;
      }

      .timeline-panel,
      .detail-panel,
      .architecture-panel,
      .next-panel {
        border: 1px solid var(--line);
        border-radius: 8px;
        background: var(--panel);
        box-shadow: var(--shadow);
      }

      .timeline-panel,
      .detail-panel,
      .architecture-panel,
      .next-panel { padding: 22px; }

      .section-heading p {
        margin-bottom: 0;
        color: var(--muted);
        line-height: 1.5;
      }

      .stage-list {
        display: grid;
        gap: 10px;
        margin-top: 20px;
      }

      .stage-button {
        display: grid;
        gap: 7px;
        width: 100%;
        border: 1px solid var(--line);
        border-radius: 8px;
        padding: 14px;
        background: #fbfcfb;
        color: var(--ink);
        text-align: left;
        cursor: pointer;
      }

      .stage-button:hover,
      .stage-button:focus {
        border-color: rgba(36, 104, 184, 0.45);
        outline: none;
      }

      .stage-tag,
      .detail-tag,
      .node-badge {
        width: max-content;
        border-radius: 999px;
        padding: 5px 9px;
        background: #e8f3ee;
        color: var(--green);
        font-size: 12px;
        font-weight: 800;
      }

      .stage-button small {
        color: var(--muted);
        line-height: 1.45;
      }

      .content-grid {
        display: grid;
        gap: 22px;
      }

      .detail-heading {
        border-bottom: 1px solid var(--line);
        padding-bottom: 18px;
        margin-bottom: 18px;
      }

      .detail-heading h2 { margin-top: 14px; }

      .detail-summary {
        margin-bottom: 0;
        color: var(--muted);
        font-size: 17px;
        line-height: 1.5;
      }

      .detail-block ul,
      .next-list {
        display: grid;
        gap: 10px;
        margin: 0;
        padding-left: 20px;
      }

      .detail-block li,
      .detail-block p,
      .next-list span {
        color: var(--muted);
        line-height: 1.55;
      }

      .emphasis {
        margin-top: 18px;
        border-left: 4px solid var(--green);
        padding-left: 14px;
      }

      .arch-grid {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
        gap: 12px;
        margin-top: 20px;
      }

      .arch-node {
        min-height: 170px;
        border: 1px solid var(--line);
        border-top: 5px solid var(--green);
        border-radius: 8px;
        padding: 15px;
        background: #fbfcfb;
      }

      .arch-node h3 { margin-top: 16px; }
      .arch-node p { margin-bottom: 0; color: var(--muted); line-height: 1.5; }

      .node-badge.amf { background: #e8f0fb; color: var(--blue); }
      .node-badge.smf { background: #fbf0dc; color: var(--amber); }
      .node-badge.k8s { background: #f7eaea; color: var(--red); }

      .next-list {
        margin-top: 20px;
      }

      .next-list li strong {
        display: block;
        margin-bottom: 4px;
      }

      @media (max-width: 1100px) {
        .hero,
        main {
          grid-template-columns: 1fr;
        }
      }

      @media (max-width: 760px) {
        .metrics,
        .arch-grid {
          grid-template-columns: 1fr;
        }

        h1 { font-size: 36px; }
      }
    </style>
  </head>
  <body>
    {%app_entry%}
    <footer>
      {%config%}
      {%scripts%}
      {%renderer%}
    </footer>
  </body>
</html>
"""


if __name__ == "__main__":
    app.run(host="127.0.0.1", port=8050, debug=True)

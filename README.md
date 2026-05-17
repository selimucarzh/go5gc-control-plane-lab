# go5gc-control-plane-lab

`go5gc-control-plane-lab` is a small, extensible Go workspace for 5GC control-plane exercises.

## MVP

The initial services:

- Generate one or more UEs
- Create IMSI/SUPI identities
- Send registration requests
- Trigger PDU session requests
- Log response details

`ue-sim`

- UE simulator CLI
- Generates one or more UEs
- Sends requests to `cp-stub` or similar control-plane endpoints

`cp-stub`

- Small Go control-plane stub service
- Exposes `POST /registration`
- Exposes `POST /pdu-sessions`
- Keeps UE and PDU session state in memory
- Rejects PDU session creation for unregistered UEs

## Directory structure

```text
cmd/ue-sim                 CLI entry point
cmd/cp-stub                Control-plane stub entry point
cmd/amf                    AMF service entry point
cmd/smf                    SMF service entry point
internal/cpstub            HTTP handlers and in-memory state
internal/config            Runtime configuration
internal/controlplane      HTTP and mock control-plane clients
internal/identity          IMSI/SUPI generation
internal/sim               UE simulation flow
internal/amf               AMF handlers and UE context state
internal/smf               SMF handlers, AMF client, and session state
internal/models            Shared AMF/SMF request and response models
```

## AMF v1 learning path

Alongside the combined `cp-stub` path, the repository also contains a smaller AMF-first path:

```text
UE simulator -> AMF registration endpoint -> in-memory UE context map
```

AMF components:

- `cmd/amf`: starts the AMF HTTP service
- `internal/amf`: registration handler and UE context map
- `internal/models`: registration request/response contract

Run it:

```powershell
go run ./cmd/amf
curl -X POST http://127.0.0.1:8081/registration `
  -H "Content-Type: application/json" `
  -d '{"supi":"imsi-001010000000001","plmn_id":"00101","access_type":"3GPP_ACCESS"}'
curl http://127.0.0.1:8081/ues/imsi-001010000000001
```

This path keeps the learning sequence explicit: AMF first, then SMF.

## SMF v1 learning path

SMF runs as a separate service and verifies that the UE is registered in AMF before creating a PDU session:

```text
UE -> AMF registration
UE -> SMF PDU session creation -> AMF UE lookup
```

SMF components:

- `cmd/smf`: starts the SMF HTTP service
- `internal/smf`: PDU session handler, AMF HTTP client, and session context map
- `internal/models`: PDU session request/response contracts

Run order:

```powershell
go run ./cmd/amf
go run ./cmd/smf
curl -X POST http://127.0.0.1:8081/registration `
  -H "Content-Type: application/json" `
  -d '{"supi":"imsi-001010000000001","plmn_id":"00101","access_type":"3GPP_ACCESS"}'
curl -X POST http://127.0.0.1:8082/pdu-sessions `
  -H "Content-Type: application/json" `
  -d '{"supi":"imsi-001010000000001","session_id":10,"dnn":"internet","s_nssai":{"sst":1,"sd":"010203"}}'
```

## Running the original stub path

### cp-stub

Start the small control-plane service first:

```powershell
go run ./cmd/cp-stub --addr :8080
```

Health check:

```powershell
curl http://127.0.0.1:8080/healthz
```

### ue-sim mock mode

To see the flow end to end without a real control-plane service:

```powershell
go run ./cmd/ue-sim --mode mock --count 3
```

### ue-sim HTTP mode

To send requests to a real endpoint:

```powershell
go run ./cmd/ue-sim --mode http --base-url http://127.0.0.1:8080 --count 2
```

`ue-sim` uses the following endpoints, and `cp-stub` provides them:

- `POST /registration`
- `POST /pdu-sessions`

## Example request bodies

Registration:

```json
{
  "imsi": "001010000000001",
  "supi": "imsi-001010000000001",
  "gnb_id": "gNB-001"
}
```

PDU session:

```json
{
  "imsi": "001010000000001",
  "supi": "imsi-001010000000001",
  "session_id": 10,
  "dnn": "internet",
  "s_nssai": {
    "sst": 1,
    "sd": "010203"
  }
}
```

## Notes

- The default mode is `mock`, so the service can be tried independently during the first setup.
- `cp-stub` currently runs as a stateful lab service and stores data in memory.
- HTTP mode provides a small integration surface that can later connect to real AMF/SMF lab services.

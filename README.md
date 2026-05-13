# go5gc-control-plane-lab

`go5gc-control-plane-lab` 5GC control-plane egzersizleri için küçük ve genişletilebilir bir Go çalışma alanıdır.

## MVP

İlk servisler:

- Bir veya birden fazla UE üretir
- IMSI/SUPI oluşturur
- Registration request gönderir
- PDU session request tetikler
- Response bilgilerini loglar

`ue-sim`

- UE simulator CLI
- Bir veya birden fazla UE üretir
- `cp-stub` veya benzeri control-plane endpointlerine request yollar

`cp-stub`

- Küçük bir Go control-plane stub servisi
- `POST /registration` endpointi sunar
- `POST /pdu-sessions` endpointi sunar
- Memory içinde UE ve PDU session state tutar
- Register olmadan PDU session açılmasını reddeder

## Klasör yapısı

```text
cmd/ue-sim                 CLI giriş noktası
cmd/cp-stub                Control-plane stub girişi
internal/cpstub            HTTP handler ve in-memory state
internal/config            Çalışma zamanı ayarları
internal/controlplane      HTTP ve mock control-plane istemcileri
internal/identity          IMSI/SUPI üretimi
internal/sim               UE simülasyon akışı
```

## Çalıştırma

### cp-stub

Önce küçük control-plane servisini başlat:

```powershell
go run ./cmd/cp-stub --addr :8080
```

Health check:

```powershell
curl http://127.0.0.1:8080/healthz
```

### ue-sim mock mod

Gerçek bir control-plane servisi olmadan akışı uçtan uca görmek için:

```powershell
go run ./cmd/ue-sim --mode mock --count 3
```

### ue-sim HTTP mod

Gerçek bir endpoint'e istek göndermek için:

```powershell
go run ./cmd/ue-sim --mode http --base-url http://127.0.0.1:8080 --count 2
```

`ue-sim` şu endpoint'leri kullanır ve `cp-stub` bu endpoint'leri sağlar:

- `POST /registration`
- `POST /pdu-sessions`

## Örnek request body

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

## Notlar

- Varsayılan mod `mock` olarak gelir; bu sayede servis ilk kurulumda bağımsız şekilde denenebilir.
- `cp-stub` şimdilik stateful bir lab servisi olarak çalışır ve in-memory saklama yapar.
- HTTP mod daha sonra gerçek AMF/SMF lab servislerine bağlanmak için bırakılmış küçük bir entegrasyon yüzeyi sunar.

package identity

import "fmt"

type UE struct {
	Index int
	IMSI  string
	SUPI  string
}

func Generate(index int, mcc, mnc string, msin uint64) UE {
	imsi := fmt.Sprintf("%s%s%010d", mcc, mnc, msin)
	return UE{
		Index: index,
		IMSI:  imsi,
		SUPI:  "imsi-" + imsi,
	}
}

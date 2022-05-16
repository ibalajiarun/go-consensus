package peerpb

import (
	"fmt"
	"strconv"
)

type PeerID uint64

func (a *Algorithm) UnmarshalJSON(b []byte) error {
	key, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	val, ok := Algorithm_value[key]
	if !ok {
		return fmt.Errorf("Algorithm not supported %s. Supported values are %v", string(b), Algorithm_value)
	}
	*a = Algorithm(val)
	return nil
}

func (a Algorithm) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(a.String())), nil
}

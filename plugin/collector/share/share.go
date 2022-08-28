package share

import (
	"strconv"

	"github.com/chriskaliX/SDK"
)

var Sandbox SDK.ISandbox

func ParseUint32(input string) (output uint32) {
	_output, err := strconv.ParseUint(input, 10, 32)
	if err != nil {
		return
	}
	output = uint32(_output)
	return
}

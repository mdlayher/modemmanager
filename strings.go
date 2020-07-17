// Code generated by "stringer -type=PortType -output strings.go"; DO NOT EDIT.

package modemmanager

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PortTypeUnknown-1]
	_ = x[PortTypeNet-2]
	_ = x[PortTypeAT-3]
	_ = x[PortTypeQCDM-4]
	_ = x[PortTypeGPS-5]
	_ = x[PortTypeQMI-6]
	_ = x[PortTypeMBIM-7]
	_ = x[PortTypeAudio-8]
}

const _PortType_name = "PortTypeUnknownPortTypeNetPortTypeATPortTypeQCDMPortTypeGPSPortTypeQMIPortTypeMBIMPortTypeAudio"

var _PortType_index = [...]uint8{0, 15, 26, 36, 48, 59, 70, 82, 95}

func (i PortType) String() string {
	i -= 1
	if i < 0 || i >= PortType(len(_PortType_index)-1) {
		return "PortType(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _PortType_name[_PortType_index[i]:_PortType_index[i+1]]
}

// Code generated by "stringer -type=PortType,PowerState,State -output strings.go"; DO NOT EDIT.

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
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PowerStateUnknown-0]
	_ = x[PowerStateOff-1]
	_ = x[PowerStateLow-2]
	_ = x[PowerStateOn-3]
}

const _PowerState_name = "PowerStateUnknownPowerStateOffPowerStateLowPowerStateOn"

var _PowerState_index = [...]uint8{0, 17, 30, 43, 55}

func (i PowerState) String() string {
	if i < 0 || i >= PowerState(len(_PowerState_index)-1) {
		return "PowerState(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _PowerState_name[_PowerState_index[i]:_PowerState_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[StateFailed - -1]
	_ = x[StateUnknown-0]
	_ = x[StateInitializing-1]
	_ = x[StateLocked-2]
	_ = x[StateDisabled-3]
	_ = x[StateDisabling-4]
	_ = x[StateEnabling-5]
	_ = x[StateEnabled-6]
	_ = x[StateSearching-7]
	_ = x[StateRegistered-8]
	_ = x[StateDisconnecting-9]
	_ = x[StateConnecting-10]
	_ = x[StateConnected-11]
}

const _State_name = "StateFailedStateUnknownStateInitializingStateLockedStateDisabledStateDisablingStateEnablingStateEnabledStateSearchingStateRegisteredStateDisconnectingStateConnectingStateConnected"

var _State_index = [...]uint8{0, 11, 23, 40, 51, 64, 78, 91, 103, 117, 132, 150, 165, 179}

func (i State) String() string {
	i -= -1
	if i < 0 || i >= State(len(_State_index)-1) {
		return "State(" + strconv.FormatInt(int64(i+-1), 10) + ")"
	}
	return _State_name[_State_index[i]:_State_index[i+1]]
}

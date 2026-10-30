// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

// Error is an error code that the server answers a command with. It is the
// underlying cause of the error Client.Request returns for a rejected command:
// check it with errors.Is(err, proto.ErrNoSuchEntity).
type Error uint32

// The Err constants are the error codes of the PulseAudio protocol.
const (
	ok Error = iota
	ErrAccessDenied
	ErrUnknownCommand
	ErrInvalidArgument
	ErrEntityExists
	ErrNoSuchEntity
	ErrConnectionRefused
	ErrProtocolError
	ErrTimeout
	ErrNoAuthenticationKey
	ErrInternalError
	ErrConnectionTerminated
	ErrEntityKilled
	ErrInvalidServer
	ErrModuleInitializationFailed
	ErrBadState
	ErrNoData
	ErrIncompatibleProtocolVersion
	ErrTooLarge
	ErrNotSupported
	ErrUnknownErrorCode
	ErrNoSuchExtension
	ErrObsoleteFunctionality
	ErrMissingImplementation
	ErrClientForked
	ErrInputOutputError
	ErrDeviceOrResourceBusy
)

// Error returns the description of the code.
func (e Error) Error() string {
	switch e {
	case ok:
		return "pulseaudio: ok"
	case ErrAccessDenied:
		return "pulseaudio: access denied"
	case ErrUnknownCommand:
		return "pulseaudio: unknown command"
	case ErrInvalidArgument:
		return "pulseaudio: invalid argument"
	case ErrEntityExists:
		return "pulseaudio: entity exists"
	case ErrNoSuchEntity:
		return "pulseaudio: no such entity"
	case ErrConnectionRefused:
		return "pulseaudio: connection refused"
	case ErrProtocolError:
		return "pulseaudio: protocol error"
	case ErrTimeout:
		return "pulseaudio: timeout"
	case ErrNoAuthenticationKey:
		return "pulseaudio: no authentication key"
	case ErrInternalError:
		return "pulseaudio: internal error"
	case ErrConnectionTerminated:
		return "pulseaudio: connection terminated"
	case ErrEntityKilled:
		return "pulseaudio: entity killed"
	case ErrInvalidServer:
		return "pulseaudio: invalid server"
	case ErrModuleInitializationFailed:
		return "pulseaudio: module initialization failed"
	case ErrBadState:
		return "pulseaudio: bad state"
	case ErrNoData:
		return "pulseaudio: no data"
	case ErrIncompatibleProtocolVersion:
		return "pulseaudio: incompatible protocol version"
	case ErrTooLarge:
		return "pulseaudio: too large"
	case ErrNotSupported:
		return "pulseaudio: not supported"
	case ErrUnknownErrorCode:
		return "pulseaudio: unknown error code"
	case ErrNoSuchExtension:
		return "pulseaudio: no such extension"
	case ErrObsoleteFunctionality:
		return "pulseaudio: obsolete functionality"
	case ErrMissingImplementation:
		return "pulseaudio: missing implementation"
	case ErrClientForked:
		return "pulseaudio: client forked"
	case ErrInputOutputError:
		return "pulseaudio: input/output error"
	case ErrDeviceOrResourceBusy:
		return "pulseaudio: device or resource busy"
	}
	return "pulseaudio: invalid error code"
}

// Copied from github.com/jfreymuth/pulse v0.1.3 and changed for Radio Tutor. MIT License, Copyright (c) 2019 Johann Freymuth: see internal/core/pulse/LICENSE.

package proto

// The Op constants are the numbers of the commands of the PulseAudio native
// protocol. The ones marked "server -> client" are messages the server sends.
const (
	OpError   = 0
	OpTimeout = 1
	OpReply   = 2

	OpCreatePlaybackStream = 3
	OpDeletePlaybackStream = 4
	OpCreateRecordStream   = 5
	OpDeleteRecordStream   = 6

	OpExit          = 7
	OpAuth          = 8
	OpSetClientName = 9

	OpLookupSink          = 10
	OpLookupSource        = 11
	OpDrainPlaybackStream = 12
	OpStat                = 13
	OpGetPlaybackLatency  = 14
	OpCreateUploadStream  = 15
	OpDeleteUploadStream  = 16
	OpFinishUploadStream  = 17
	OpPlaySample          = 18
	OpRemoveSample        = 19

	OpGetServerInfo           = 20
	OpGetSinkInfo             = 21
	OpGetSinkInfoList         = 22
	OpGetSourceInfo           = 23
	OpGetSourceInfoList       = 24
	OpGetModuleInfo           = 25
	OpGetModuleInfoList       = 26
	OpGetClientInfo           = 27
	OpGetClientInfoList       = 28
	OpGetSinkInputInfo        = 29
	OpGetSinkInputInfoList    = 30
	OpGetSourceOutputInfo     = 31
	OpGetSourceOutputInfoList = 32
	OpGetSampleInfo           = 33
	OpGetSampleInfoList       = 34
	OpSubscribe               = 35

	OpSetSinkVolume         = 36
	OpSetSinkInputVolume    = 37
	OpSetSourceVolume       = 38
	OpSetSinkMute           = 39
	OpSetSourceMute         = 40
	OpCorkPlaybackStream    = 41
	OpFlushPlaybackStream   = 42
	OpTriggerPlaybackStream = 43

	OpSetDefaultSink        = 44
	OpSetDefaultSource      = 45
	OpSetPlaybackStreamName = 46
	OpSetRecordStreamName   = 47
	OpKillClient            = 48
	OpKillSinkInput         = 49
	OpKillSourceOutput      = 50

	OpLoadModule   = 51
	OpUnloadModule = 52

	// 4 obsolete commands

	OpGetRecordLatency     = 57
	OpCorkRecordStream     = 58
	OpFlushRecordStream    = 59
	OpPrebufPlaybackStream = 60

	OpRequest              = 61 // server -> client
	OpOverflow             = 62 // server -> client
	OpUnderflow            = 63 // server -> client
	OpPlaybackStreamKilled = 64 // server -> client
	OpRecordStreamKilled   = 65 // server -> client
	OpSubscribeEvent       = 66 // server -> client

	OpMoveSinkInput                  = 67
	OpMoveSourceOutput               = 68
	OpSetSinkInputMute               = 69
	OpSuspendSink                    = 70
	OpSuspendSource                  = 71
	OpSetPlaybackStreamBufferAttr    = 72
	OpSetRecordStreamBufferAttr      = 73
	OpUpdatePlaybackStreamSampleRate = 74
	OpUpdateRecordStreamSampleRate   = 75

	OpPlaybackStreamSuspended = 76 // server -> client
	OpRecordStreamSuspended   = 77 // server -> client
	OpPlaybackStreamMoved     = 78 // server -> client
	OpRecordStreamMoved       = 79 // server -> client

	OpUpdateRecordStreamProplist   = 80
	OpUpdatePlaybackStreamProplist = 81
	OpUpdateClientProplist         = 82
	OpRemoveRecordStreamProplist   = 83
	OpRemovePlaybackStreamProplist = 84
	OpRemoveClientProplist         = 85

	OpStarted = 86 // server -> client

	OpExtension = 87

	OpGetCardInfo     = 88
	OpGetCardInfoList = 89
	OpSetCardProfile  = 90

	OpClientEvent               = 91 // server -> client
	OpPlaybackStreamEvent       = 92 // server -> client
	OpRecordStreamEvent         = 93 // server -> client
	OpPlaybackBufferAttrChanged = 94 // server -> client
	OpRecordBufferAttrChanged   = 95 // server -> client

	OpSetSinkPort           = 96
	OpSetSourcePort         = 97
	OpSetSourceOutputVolume = 98
	OpSetSourceOutputMute   = 99

	OpSetPortLatencyOffset = 100

	OpEnableSRBChannel  = 101
	OpDisableSRBChannel = 102

	OpRegisterMemfdShmid = 103
)

// RequestArgs is a command that Client.Request can send: one of the request
// types of this package.
type RequestArgs interface{ command() uint32 }

// Reply is the reply to a command: one of the reply types of this package.
type Reply interface{ IsReplyTo() uint32 }

// CreatePlaybackStream is the protocol command OpCreatePlaybackStream.
type CreatePlaybackStream struct {
	SampleSpec
	ChannelMap ChannelMap
	SinkIndex  uint32
	SinkName   string

	BufferMaxLength       uint32
	Corked                bool
	BufferTargetLength    uint32
	BufferPrebufferLength uint32
	BufferMinimumRequest  uint32

	SyncID uint32

	ChannelVolumes ChannelVolumes

	NoRemap      bool `pulse:"12"`
	NoRemix      bool `pulse:"12"`
	FixFormat    bool `pulse:"12"`
	FixRate      bool `pulse:"12"`
	FixChannels  bool `pulse:"12"`
	NoMove       bool `pulse:"12"`
	VariableRate bool `pulse:"12"`

	Muted         bool     `pulse:"13"`
	AdjustLatency bool     `pulse:"13"`
	Properties    PropList `pulse:"13"`

	VolumeSet     bool `pulse:"14"`
	EarlyRequests bool `pulse:"14"`

	MutedSet               bool `pulse:"15"`
	DontInhibitAutoSuspend bool `pulse:"15"`
	FailOnSuspend          bool `pulse:"15"`

	RelativeVolume bool `pulse:"17"`

	Passthrough bool `pulse:"18"`

	Formats []FormatInfo `pulse:"21"`
}

// CreatePlaybackStreamReply is the server's reply to the command OpCreatePlaybackStream.
type CreatePlaybackStreamReply struct {
	StreamIndex    uint32
	SinkInputIndex uint32
	Missing        uint32

	BufferMaxLength       uint32 `pulse:"9"`
	BufferTargetLength    uint32 `pulse:"9"`
	BufferPrebufferLength uint32 `pulse:"9"`
	BufferMinimumRequest  uint32 `pulse:"9"`

	SampleSpec `pulse:"12"`
	ChannelMap []byte `pulse:"12"`

	SinkIndex     uint32 `pulse:"12"`
	SinkName      string `pulse:"12"`
	SinkSuspended bool   `pulse:"12"`

	SinkLatency Microseconds `pulse:"13"`

	FormatInfo `pulse:"21"`
}

// DeletePlaybackStream is the protocol command OpDeletePlaybackStream.
type DeletePlaybackStream struct{ StreamIndex uint32 }

// CreateRecordStream is the protocol command OpCreateRecordStream.
type CreateRecordStream struct {
	SampleSpec
	ChannelMap      ChannelMap
	SourceIndex     uint32
	SourceName      string
	BufferMaxLength uint32
	Corked          bool
	BufferFragSize  uint32

	NoRemap      bool `pulse:"12"`
	NoRemix      bool `pulse:"12"`
	FixFormat    bool `pulse:"12"`
	FixRate      bool `pulse:"12"`
	FixChannels  bool `pulse:"12"`
	NoMove       bool `pulse:"12"`
	VariableRate bool `pulse:"12"`

	PeakDetect         bool     `pulse:"13"`
	AdjustLatency      bool     `pulse:"13"`
	Properties         PropList `pulse:"13"`
	DirectOnInputIndex uint32   `pulse:"13"`

	EarlyRequests bool `pulse:"14"`

	DontInhibitAutoSuspend bool `pulse:"15"`
	FailOnSuspend          bool `pulse:"15"`

	Formats        []FormatInfo   `pulse:"22"`
	ChannelVolumes ChannelVolumes `pulse:"22"`
	Muted          bool           `pulse:"22"`
	VolumeSet      bool           `pulse:"22"`
	MutedSet       bool           `pulse:"22"`
	RelativeVolume bool           `pulse:"22"`
	Passthrough    bool           `pulse:"22"`
}

// CreateRecordStreamReply is the server's reply to the command OpCreateRecordStream.
type CreateRecordStreamReply struct {
	StreamIndex       uint32
	SourceOutputIndex uint32

	BufferMaxLength uint32 `pulse:"9"`
	BufferFragSize  uint32 `pulse:"9"`

	SampleSpec      `pulse:"12"`
	ChannelMap      ChannelMap `pulse:"12"`
	SourceIndex     uint32     `pulse:"12"`
	SourceName      string     `pulse:"12"`
	SourceSuspended bool       `pulse:"12"`

	SourceLatency Microseconds `pulse:"13"`

	FormatInfo `pulse:"22"`
}

// DeleteRecordStream is the protocol command OpDeleteRecordStream.
type DeleteRecordStream struct{ StreamIndex uint32 }

// Exit is the protocol command OpExit.
type Exit struct{}

// Auth is the protocol command OpAuth.
type Auth struct {
	Version Version
	Cookie  []byte
}

// AuthReply is the server's reply to the command OpAuth.
type AuthReply struct {
	Version Version
}

// SetClientName is the protocol command OpSetClientName.
type SetClientName struct {
	Props PropList
}

// SetClientNameReply is the server's reply to the command OpSetClientName.
type SetClientNameReply struct {
	ClientIndex uint32
}

// LookupSink is the protocol command OpLookupSink.
type LookupSink struct{ SinkName string }

// LookupSinkReply is the server's reply to the command OpLookupSink.
type LookupSinkReply struct{ SinkIndex uint32 }

// LookupSource is the protocol command OpLookupSource.
type LookupSource struct{ SourceName string }

// LookupSourceReply is the server's reply to the command OpLookupSource.
type LookupSourceReply struct{ SourceIndex uint32 }

// DrainPlaybackStream is the protocol command OpDrainPlaybackStream.
type DrainPlaybackStream struct {
	StreamIndex uint32
}

// Stat is the protocol command OpStat.
type Stat struct{}

// StatReply is the server's reply to the command OpStat.
type StatReply struct {
	NumAllocated    uint32
	AllocatedSize   uint32
	NumAccumulated  uint32
	AccumulatedSize uint32
	SampleCacheSize uint32
}

// GetPlaybackLatency is the protocol command OpGetPlaybackLatency.
type GetPlaybackLatency struct {
	StreamIndex uint32
	Time        Time
}

// GetPlaybackLatencyReply is the server's reply to the command OpGetPlaybackLatency.
type GetPlaybackLatencyReply struct {
	Latency     Microseconds
	Unused      Microseconds // always 0
	Running     bool
	RequestTime Time
	ReplyTime   Time
	WriteIndex  int64
	ReadIndex   int64

	UnderrunFor uint64 `pulse:"13"`
	PlayingFor  uint64 `pulse:"13"`
}

// GetRecordLatency is the protocol command OpGetRecordLatency.
type GetRecordLatency struct {
	StreamIndex uint32
	Time        Time
}

// GetRecordLatencyReply is the server's reply to the command OpGetRecordLatency.
type GetRecordLatencyReply struct {
	MonitorLatency Microseconds
	Latency        Microseconds
	Running        bool
	RequestTime    Time
	ReplyTime      Time
	WriteIndex     int64
	ReadIndex      int64
}

// CreateUploadStream is the protocol command OpCreateUploadStream.
type CreateUploadStream struct {
	Name string
	SampleSpec
	ChannelMap ChannelMap
	Length     uint32

	Properties PropList `pulse:"13"`
}

// CreateUploadStreamReply is the server's reply to the command OpCreateUploadStream.
type CreateUploadStreamReply struct {
	StreamIndex uint32
	Length      uint32
}

// DeleteUploadStream is the protocol command OpDeleteUploadStream.
type DeleteUploadStream struct{ StreamIndex uint32 }

// FinishUploadStream is the protocol command OpFinishUploadStream.
type FinishUploadStream struct {
	StreamIndex uint32
}

// PlaySample is the protocol command OpPlaySample.
type PlaySample struct {
	SinkIndex uint32
	SinkName  string
	Volume    uint32
	Name      string

	Properties PropList `pulse:"13"`
}

// RemoveSample is the protocol command OpRemoveSample.
type RemoveSample struct {
	Name string
}

// GetServerInfo is the protocol command OpGetServerInfo.
type GetServerInfo struct{}

// GetServerInfoReply is the server's reply to the command OpGetServerInfo.
type GetServerInfoReply struct {
	PackageName    string
	PackageVersion string
	Username       string
	Hostname       string

	DefaultSampleSpec SampleSpec
	DefaultSinkName   string
	DefaultSourceName string

	Cookie uint32

	DefaultChannelMap ChannelMap `pulse:"15"`
}

// GetSinkInfo is the protocol command OpGetSinkInfo.
type GetSinkInfo struct {
	SinkIndex uint32
	SinkName  string
}

// GetSinkInfoReply is the server's reply to the command OpGetSinkInfo.
type GetSinkInfoReply struct {
	SinkIndex uint32
	SinkName  string
	Device    string
	SampleSpec
	ChannelMap         ChannelMap
	ModuleIndex        uint32
	ChannelVolumes     ChannelVolumes
	Mute               bool
	MonitorSourceIndex uint32
	MonitorSourceName  string
	Latency            Microseconds
	Driver             string
	Flags              uint32

	Properties       PropList     `pulse:"13"`
	RequestedLatency Microseconds `pulse:"13"`

	BaseVolume     Volume `pulse:"15"`
	State          uint32 `pulse:"15"`
	NumVolumeSteps uint32 `pulse:"15"`
	CardIndex      uint32 `pulse:"15"`

	Ports []struct {
		Name        string
		Description string
		Priority    uint32
		Available   uint32 `pulse:"24"`
	} `pulse:"16"`
	ActivePortName string `pulse:"16"`

	Formats []FormatInfo `pulse:"21"`
}

// GetSourceInfo is the protocol command OpGetSourceInfo.
type GetSourceInfo struct {
	SourceIndex uint32
	SourceName  string
}

// GetSourceInfoReply is the server's reply to the command OpGetSourceInfo.
type GetSourceInfoReply struct {
	SourceIndex uint32
	SourceName  string
	Device      string
	SampleSpec
	ChannelMap         ChannelMap
	ModuleIndex        uint32
	ChannelVolumes     ChannelVolumes
	Mute               bool
	MonitorSourceIndex uint32
	MonitorSourceName  string
	Latency            Microseconds
	Driver             string
	Flags              uint32

	Properties       PropList     `pulse:"13"`
	RequestedLatency Microseconds `pulse:"13"`

	BaseVolume     Volume `pulse:"15"`
	State          uint32 `pulse:"15"`
	NumVolumeSteps uint32 `pulse:"15"`
	CardIndex      uint32 `pulse:"15"`

	Ports []struct {
		Name        string
		Description string
		Priority    uint32
		Available   uint32 `pulse:"24"`
	} `pulse:"16"`
	ActivePortName string `pulse:"16"`

	Formats []FormatInfo `pulse:"21"`
}

// GetClientInfo is the protocol command OpGetClientInfo.
type GetClientInfo struct{ ClientIndex uint32 }

// GetClientInfoReply is the server's reply to the command OpGetClientInfo.
type GetClientInfoReply struct {
	ClientIndex uint32
	Application string
	ModuleIndex uint32
	Driver      string

	Properties PropList `pulse:"13"`
}

// GetCardInfo is the protocol command OpGetCardInfo.
type GetCardInfo struct{ CardIndex uint32 }

// GetCardInfoReply is the server's reply to the command OpGetCardInfo.
type GetCardInfoReply struct {
	CardIndex   uint32
	CardName    string
	ModuleIndex uint32
	Driver      string

	Profiles []struct {
		Name        string
		Description string
		NumSinks    uint32
		NumSources  uint32
		Priority    uint32
		Available   uint32 `pulse:"29"`
	}
	ActiveProfileName string
	Properties        PropList

	Ports []struct {
		Name        string
		Description string
		Priority    uint32
		Available   uint32
		Direction   byte
		Properties  PropList
		Profiles    []struct {
			Name string
		}
		LatencyOffset int64 `pulse:"27"`
	} `pulse:"26"`
}

// GetModuleInfo is the protocol command OpGetModuleInfo.
type GetModuleInfo struct{ ModuleIndex uint32 }

// GetModuleInfoReply is the server's reply to the command OpGetModuleInfo.
type GetModuleInfoReply struct {
	ModuleIndex uint32
	ModuleName  string
	ModuleArgs  string
	Users       uint32

	Properties PropList `pulse:"15"`
	AutoLoad   bool     `pulse:"<15"`
}

// GetSinkInputInfo is the protocol command OpGetSinkInputInfo.
type GetSinkInputInfo struct{ SinkInputIndex uint32 }

// GetSinkInputInfoReply is the server's reply to the command OpGetSinkInputInfo.
type GetSinkInputInfoReply struct {
	SinkInputIndex uint32
	MediaName      string
	ModuleIndex    uint32
	ClientIndex    uint32
	SinkIndex      uint32
	SampleSpec
	ChannelMap     ChannelMap
	ChannelVolumes ChannelVolumes

	SinkInputLatency Microseconds
	SinkLatency      Microseconds
	ResampleMethod   string
	Driver           string

	Muted bool `pulse:"11"`

	Properties PropList `pulse:"13"`

	Corked bool `pulse:"19"`

	VolumeReadable bool `pulse:"20"`
	VolumeWritable bool `pulse:"20"`

	FormatInfo `pulse:"21"`
}

// GetSourceOutputInfo is the protocol command OpGetSourceOutputInfo.
type GetSourceOutputInfo struct{ SourceOutpuIndex uint32 }

// GetSourceOutputInfoReply is the server's reply to the command OpGetSourceOutputInfo.
type GetSourceOutputInfoReply struct {
	SourceOutpuIndex uint32
	MediaName        string
	ModuleIndex      uint32
	ClientIndex      uint32
	SourceIndex      uint32
	SampleSpec
	ChannelMap ChannelMap

	SourceOutpuLatency Microseconds
	SourceLatency      Microseconds
	ResampleMethod     string
	Driver             string

	Properties PropList `pulse:"13"`

	Corked bool `pulse:"19"`

	ChannelVolumes ChannelVolumes `pulse:"22"`
	Muted          bool           `pulse:"22"`
	VolumeReadable bool           `pulse:"22"`
	VolumeWritable bool           `pulse:"22"`
	FormatInfo     `pulse:"22"`
}

// GetSampleInfo is the protocol command OpGetSampleInfo.
type GetSampleInfo struct {
	SampleIndex uint32
	SampleName  string
}

// GetSampleInfoReply is the server's reply to the command OpGetSampleInfo.
type GetSampleInfoReply struct {
	SampleIndex    uint32
	SampleName     string
	ChannelVolumes ChannelVolumes
	Duration       Microseconds
	SampleSpec
	ChannelMap ChannelMap
	Length     uint32
	Lazy       bool
	Filename   string

	Properties PropList `pulse:"13"`
}

// GetSinkInfoList is the protocol command OpGetSinkInfoList.
type GetSinkInfoList struct{}

// GetSourceInfoList is the protocol command OpGetSourceInfoList.
type GetSourceInfoList struct{}

// GetModuleInfoList is the protocol command OpGetModuleInfoList.
type GetModuleInfoList struct{}

// GetClientInfoList is the protocol command OpGetClientInfoList.
type GetClientInfoList struct{}

// GetCardInfoList is the protocol command OpGetCardInfoList.
type GetCardInfoList struct{}

// GetSinkInputInfoList is the protocol command OpGetSinkInputInfoList.
type GetSinkInputInfoList struct{}

// GetSourceOutputInfoList is the protocol command OpGetSourceOutputInfoList.
type GetSourceOutputInfoList struct{}

// GetSampleInfoList is the protocol command OpGetSampleInfoList.
type GetSampleInfoList struct{}

// GetSinkInfoListReply is the server's reply to the command OpGetSinkInfoList.
type GetSinkInfoListReply []*GetSinkInfoReply

// GetSourceInfoListReply is the server's reply to the command OpGetSourceInfoList.
type GetSourceInfoListReply []*GetSourceInfoReply

// GetModuleInfoListReply is the server's reply to the command OpGetModuleInfoList.
type GetModuleInfoListReply []*GetModuleInfoReply

// GetClientInfoListReply is the server's reply to the command OpGetClientInfoList.
type GetClientInfoListReply []*GetClientInfoReply

// GetCardInfoListReply is the server's reply to the command OpGetCardInfoList.
type GetCardInfoListReply []*GetCardInfoReply

// GetSinkInputInfoListReply is the server's reply to the command OpGetSinkInputInfoList.
type GetSinkInputInfoListReply []*GetSinkInputInfoReply

// GetSourceOutputInfoListReply is the server's reply to the command OpGetSourceOutputInfoList.
type GetSourceOutputInfoListReply []*GetSourceOutputInfoReply

// GetSampleInfoListReply is the server's reply to the command OpGetSampleInfoList.
type GetSampleInfoListReply []*GetSampleInfoReply

// Subscribe is the protocol command OpSubscribe.
type Subscribe struct{ Mask SubscriptionMask }

// SetSinkVolume is the protocol command OpSetSinkVolume.
type SetSinkVolume struct {
	SinkIndex      uint32
	SinkName       string
	ChannelVolumes ChannelVolumes
}

// SetSourceVolume is the protocol command OpSetSourceVolume.
type SetSourceVolume struct {
	SourceIndex    uint32
	SourceName     string
	ChannelVolumes ChannelVolumes
}

// SetSinkInputVolume is the protocol command OpSetSinkInputVolume.
type SetSinkInputVolume struct {
	SinkInputIndex uint32
	ChannelVolumes ChannelVolumes
}

// SetSourceOutputVolume is the protocol command OpSetSourceOutputVolume.
type SetSourceOutputVolume struct {
	SourceOutputIndex uint32
	ChannelVolumes    ChannelVolumes
}

// SetSinkMute is the protocol command OpSetSinkMute.
type SetSinkMute struct {
	SinkIndex uint32
	SinkName  string
	Mute      bool
}

// SetSourceMute is the protocol command OpSetSourceMute.
type SetSourceMute struct {
	SourceIndex uint32
	SourceName  string
	Mute        bool
}

// SetSinkInputMute is the protocol command OpSetSinkInputMute.
type SetSinkInputMute struct {
	SinkInputIndex uint32
	Mute           bool
}

// SetSourceOutputMute is the protocol command OpSetSourceOutputMute.
type SetSourceOutputMute struct {
	SourceOutputIndex uint32
	Mute              bool
}

// CorkPlaybackStream is the protocol command OpCorkPlaybackStream.
type CorkPlaybackStream struct {
	StreamIndex uint32
	Corked      bool
}

// CorkRecordStream is the protocol command OpCorkRecordStream.
type CorkRecordStream struct {
	StreamIndex uint32
	Corked      bool
}

// FlushRecordStream is the protocol command OpFlushRecordStream.
type FlushRecordStream struct{ StreamIndex uint32 }

// TriggerPlaybackStream is the protocol command OpTriggerPlaybackStream.
type TriggerPlaybackStream struct{ StreamIndex uint32 }

// FlushPlaybackStream is the protocol command OpFlushPlaybackStream.
type FlushPlaybackStream struct{ StreamIndex uint32 }

// PrebufPlaybackStream is the protocol command OpPrebufPlaybackStream.
type PrebufPlaybackStream struct{ StreamIndex uint32 }

// SetPlaybackStreamBufferAttr is the protocol command OpSetPlaybackStreamBufferAttr.
type SetPlaybackStreamBufferAttr struct {
	StreamIndex           uint32
	BufferMaxLength       uint32
	BufferTargetLength    uint32
	BufferPrebufferLength uint32
	BufferMinimumRequest  uint32

	AdjustLatency bool `pulse:"13"`

	EarlyRequests bool `pulse:"14"`
}

// SetPlaybackStreamBufferAttrReply is the server's reply to the command OpSetPlaybackStreamBufferAttr.
type SetPlaybackStreamBufferAttrReply struct {
	BufferMaxLength       uint32
	BufferTargetLength    uint32
	BufferPrebufferLength uint32
	BufferMinimumRequest  uint32

	SinkLatency Microseconds `pulse:"13"`
}

// SetRecordStreamBufferAttr is the protocol command OpSetRecordStreamBufferAttr.
type SetRecordStreamBufferAttr struct {
	StreamIndex     uint32
	BufferMaxLength uint32
	BufferFragSize  uint32

	AdjustLatency bool `pulse:"13"`

	EarlyRequests bool `pulse:"14"`
}

// SetRecordStreamBufferAttrReply is the server's reply to the command OpSetRecordStreamBufferAttr.
type SetRecordStreamBufferAttrReply struct {
	BufferMaxLength uint32
	BufferFragSize  uint32

	SourceLatency Microseconds `pulse:"13"`
}

// UpdatePlaybackStreamSampleRate is the protocol command OpUpdatePlaybackStreamSampleRate.
type UpdatePlaybackStreamSampleRate struct {
	StreamIndex uint32
	SampleRate  uint32
}

// UpdateRecordStreamSampleRate is the protocol command OpUpdateRecordStreamSampleRate.
type UpdateRecordStreamSampleRate struct {
	StreamIndex uint32
	SampleRate  uint32
}

// UpdatePlaybackStreamProplist is the protocol command OpUpdatePlaybackStreamProplist.
type UpdatePlaybackStreamProplist struct {
	StreamIndex uint32
	Mode        uint32
	Properties  PropList
}

// UpdateRecordStreamProplist is the protocol command OpUpdateRecordStreamProplist.
type UpdateRecordStreamProplist struct {
	StreamIndex uint32
	Mode        uint32
	Properties  PropList
}

// UpdateClientProplist is the protocol command OpUpdateClientProplist.
type UpdateClientProplist struct {
	Mode       uint32
	Properties PropList
}

// RemovePlaybackStreamProplist is the protocol command OpRemovePlaybackStreamProplist.
type RemovePlaybackStreamProplist struct {
	StreamIndex uint32
	Properties  PropList // ignored
}

// RemoveRecordStreamProplist is the protocol command OpRemoveRecordStreamProplist.
type RemoveRecordStreamProplist struct {
	StreamIndex uint32
	Properties  PropList // ignored
}

// RemoveClientProplist is the protocol command OpRemoveClientProplist.
type RemoveClientProplist struct {
	Properties PropList // ignored
}

// SetDefaultSink is the protocol command OpSetDefaultSink.
type SetDefaultSink struct{ SinkName string }

// SetDefaultSource is the protocol command OpSetDefaultSource.
type SetDefaultSource struct{ SourceName string }

// SetPlaybackStreamName is the protocol command OpSetPlaybackStreamName.
type SetPlaybackStreamName struct {
	StreamIndex uint32
	Name        string
}

// SetRecordStreamName is the protocol command OpSetRecordStreamName.
type SetRecordStreamName struct {
	StreamIndex uint32
	Name        string
}

// KillSinkInput is the protocol command OpKillSinkInput.
type KillSinkInput struct{ SinkInputIndex uint32 }

// KillSourceOutput is the protocol command OpKillSourceOutput.
type KillSourceOutput struct{ SourceOutputIndex uint32 }

// KillClient is the protocol command OpKillClient.
type KillClient struct{ ClientIndex uint32 }

// LoadModule is the protocol command OpLoadModule.
type LoadModule struct {
	Name string
	Args string
}

// LoadModuleReply is the server's reply to the command OpLoadModule.
type LoadModuleReply struct {
	ModuleIndex uint32
}

// UnloadModule is the protocol command OpUnloadModule.
type UnloadModule struct{ ModuleIndex uint32 }

// MoveSinkInput is the protocol command OpMoveSinkInput.
type MoveSinkInput struct {
	SinkInputIndex uint32
	DeviceIndex    uint32
	DeviceName     string
}

// MoveSourceOutput is the protocol command OpMoveSourceOutput.
type MoveSourceOutput struct {
	SourceOutputIndex uint32
	DeviceIndex       uint32
	DeviceName        string
}

// SuspendSink is the protocol command OpSuspendSink.
type SuspendSink struct {
	SinkIndex uint32
	SinkName  string
	Suspend   bool
}

// SuspendSource is the protocol command OpSuspendSource.
type SuspendSource struct {
	SourceIndex uint32
	SourceName  string
	Suspend     bool
}

// The reply type for this command is extension-specific
type Extension struct {
	Index uint32
	Name  string
}

// SetCardProfile is the protocol command OpSetCardProfile.
type SetCardProfile struct {
	CardIndex   uint32
	CardName    string
	ProfileName string
}

// SetSinkPort is the protocol command OpSetSinkPort.
type SetSinkPort struct {
	SinkIndex uint32
	SinkName  string
	Port      string
}

// SetSourcePort is the protocol command OpSetSourcePort.
type SetSourcePort struct {
	SourceIndex uint32
	SourceName  string
	Port        string
}

// SetPortLatencyOffset is the protocol command OpSetPortLatencyOffset.
type SetPortLatencyOffset struct {
	CardIndex uint32
	CardName  string
	PortName  string
	Offset    int64
}

func (*CreatePlaybackStream) command() uint32           { return OpCreatePlaybackStream }
func (*DeletePlaybackStream) command() uint32           { return OpDeletePlaybackStream }
func (*CreateRecordStream) command() uint32             { return OpCreateRecordStream }
func (*DeleteRecordStream) command() uint32             { return OpDeleteRecordStream }
func (*Exit) command() uint32                           { return OpExit }
func (*Auth) command() uint32                           { return OpAuth }
func (*SetClientName) command() uint32                  { return OpSetClientName }
func (*LookupSink) command() uint32                     { return OpLookupSink }
func (*LookupSource) command() uint32                   { return OpLookupSource }
func (*DrainPlaybackStream) command() uint32            { return OpDrainPlaybackStream }
func (*Stat) command() uint32                           { return OpStat }
func (*GetPlaybackLatency) command() uint32             { return OpGetPlaybackLatency }
func (*CreateUploadStream) command() uint32             { return OpCreateUploadStream }
func (*DeleteUploadStream) command() uint32             { return OpDeleteUploadStream }
func (*FinishUploadStream) command() uint32             { return OpFinishUploadStream }
func (*PlaySample) command() uint32                     { return OpPlaySample }
func (*RemoveSample) command() uint32                   { return OpRemoveSample }
func (*GetServerInfo) command() uint32                  { return OpGetServerInfo }
func (*GetSinkInfo) command() uint32                    { return OpGetSinkInfo }
func (*GetSinkInfoList) command() uint32                { return OpGetSinkInfoList }
func (*GetSourceInfo) command() uint32                  { return OpGetSourceInfo }
func (*GetSourceInfoList) command() uint32              { return OpGetSourceInfoList }
func (*GetModuleInfo) command() uint32                  { return OpGetModuleInfo }
func (*GetModuleInfoList) command() uint32              { return OpGetModuleInfoList }
func (*GetClientInfo) command() uint32                  { return OpGetClientInfo }
func (*GetClientInfoList) command() uint32              { return OpGetClientInfoList }
func (*GetSinkInputInfo) command() uint32               { return OpGetSinkInputInfo }
func (*GetSinkInputInfoList) command() uint32           { return OpGetSinkInputInfoList }
func (*GetSourceOutputInfo) command() uint32            { return OpGetSourceOutputInfo }
func (*GetSourceOutputInfoList) command() uint32        { return OpGetSourceOutputInfoList }
func (*GetSampleInfo) command() uint32                  { return OpGetSampleInfo }
func (*GetSampleInfoList) command() uint32              { return OpGetSampleInfoList }
func (*Subscribe) command() uint32                      { return OpSubscribe }
func (*SetSinkVolume) command() uint32                  { return OpSetSinkVolume }
func (*SetSinkInputVolume) command() uint32             { return OpSetSinkInputVolume }
func (*SetSourceVolume) command() uint32                { return OpSetSourceVolume }
func (*SetSinkMute) command() uint32                    { return OpSetSinkMute }
func (*SetSourceMute) command() uint32                  { return OpSetSourceMute }
func (*CorkPlaybackStream) command() uint32             { return OpCorkPlaybackStream }
func (*FlushPlaybackStream) command() uint32            { return OpFlushPlaybackStream }
func (*TriggerPlaybackStream) command() uint32          { return OpTriggerPlaybackStream }
func (*SetDefaultSink) command() uint32                 { return OpSetDefaultSink }
func (*SetDefaultSource) command() uint32               { return OpSetDefaultSource }
func (*SetPlaybackStreamName) command() uint32          { return OpSetPlaybackStreamName }
func (*SetRecordStreamName) command() uint32            { return OpSetRecordStreamName }
func (*KillClient) command() uint32                     { return OpKillClient }
func (*KillSinkInput) command() uint32                  { return OpKillSinkInput }
func (*KillSourceOutput) command() uint32               { return OpKillSourceOutput }
func (*LoadModule) command() uint32                     { return OpLoadModule }
func (*UnloadModule) command() uint32                   { return OpUnloadModule }
func (*GetRecordLatency) command() uint32               { return OpGetRecordLatency }
func (*CorkRecordStream) command() uint32               { return OpCorkRecordStream }
func (*FlushRecordStream) command() uint32              { return OpFlushRecordStream }
func (*PrebufPlaybackStream) command() uint32           { return OpPrebufPlaybackStream }
func (*MoveSinkInput) command() uint32                  { return OpMoveSinkInput }
func (*MoveSourceOutput) command() uint32               { return OpMoveSourceOutput }
func (*SetSinkInputMute) command() uint32               { return OpSetSinkInputMute }
func (*SuspendSink) command() uint32                    { return OpSuspendSink }
func (*SuspendSource) command() uint32                  { return OpSuspendSource }
func (*SetPlaybackStreamBufferAttr) command() uint32    { return OpSetPlaybackStreamBufferAttr }
func (*SetRecordStreamBufferAttr) command() uint32      { return OpSetRecordStreamBufferAttr }
func (*UpdatePlaybackStreamSampleRate) command() uint32 { return OpUpdatePlaybackStreamSampleRate }
func (*UpdateRecordStreamSampleRate) command() uint32   { return OpUpdateRecordStreamSampleRate }
func (*UpdateRecordStreamProplist) command() uint32     { return OpUpdateRecordStreamProplist }
func (*UpdatePlaybackStreamProplist) command() uint32   { return OpUpdatePlaybackStreamProplist }
func (*UpdateClientProplist) command() uint32           { return OpUpdateClientProplist }
func (*RemoveRecordStreamProplist) command() uint32     { return OpRemoveRecordStreamProplist }
func (*RemovePlaybackStreamProplist) command() uint32   { return OpRemovePlaybackStreamProplist }
func (*RemoveClientProplist) command() uint32           { return OpRemoveClientProplist }
func (*Extension) command() uint32                      { return OpExtension }
func (*GetCardInfo) command() uint32                    { return OpGetCardInfo }
func (*GetCardInfoList) command() uint32                { return OpGetCardInfoList }
func (*SetCardProfile) command() uint32                 { return OpSetCardProfile }
func (*SetSinkPort) command() uint32                    { return OpSetSinkPort }
func (*SetSourcePort) command() uint32                  { return OpSetSourcePort }
func (*SetSourceOutputVolume) command() uint32          { return OpSetSourceOutputVolume }
func (*SetSourceOutputMute) command() uint32            { return OpSetSourceOutputMute }
func (*SetPortLatencyOffset) command() uint32           { return OpSetPortLatencyOffset }

// IsReplyTo returns the command CreatePlaybackStreamReply is the reply to.
func (*CreatePlaybackStreamReply) IsReplyTo() uint32 { return OpCreatePlaybackStream }

// IsReplyTo returns the command CreateRecordStreamReply is the reply to.
func (*CreateRecordStreamReply) IsReplyTo() uint32 { return OpCreateRecordStream }

// IsReplyTo returns the command AuthReply is the reply to.
func (*AuthReply) IsReplyTo() uint32 { return OpAuth }

// IsReplyTo returns the command SetClientNameReply is the reply to.
func (*SetClientNameReply) IsReplyTo() uint32 { return OpSetClientName }

// IsReplyTo returns the command LookupSinkReply is the reply to.
func (*LookupSinkReply) IsReplyTo() uint32 { return OpLookupSink }

// IsReplyTo returns the command LookupSourceReply is the reply to.
func (*LookupSourceReply) IsReplyTo() uint32 { return OpLookupSource }

// IsReplyTo returns the command StatReply is the reply to.
func (*StatReply) IsReplyTo() uint32 { return OpStat }

// IsReplyTo returns the command GetPlaybackLatencyReply is the reply to.
func (*GetPlaybackLatencyReply) IsReplyTo() uint32 { return OpGetPlaybackLatency }

// IsReplyTo returns the command CreateUploadStreamReply is the reply to.
func (*CreateUploadStreamReply) IsReplyTo() uint32 { return OpCreateUploadStream }

// IsReplyTo returns the command GetServerInfoReply is the reply to.
func (*GetServerInfoReply) IsReplyTo() uint32 { return OpGetServerInfo }

// IsReplyTo returns the command GetSinkInfoReply is the reply to.
func (*GetSinkInfoReply) IsReplyTo() uint32 { return OpGetSinkInfo }

// IsReplyTo returns the command GetSinkInfoListReply is the reply to.
func (*GetSinkInfoListReply) IsReplyTo() uint32 { return OpGetSinkInfoList }

// IsReplyTo returns the command GetSourceInfoReply is the reply to.
func (*GetSourceInfoReply) IsReplyTo() uint32 { return OpGetSourceInfo }

// IsReplyTo returns the command GetSourceInfoListReply is the reply to.
func (*GetSourceInfoListReply) IsReplyTo() uint32 { return OpGetSourceInfoList }

// IsReplyTo returns the command GetModuleInfoReply is the reply to.
func (*GetModuleInfoReply) IsReplyTo() uint32 { return OpGetModuleInfo }

// IsReplyTo returns the command GetModuleInfoListReply is the reply to.
func (*GetModuleInfoListReply) IsReplyTo() uint32 { return OpGetModuleInfoList }

// IsReplyTo returns the command GetClientInfoReply is the reply to.
func (*GetClientInfoReply) IsReplyTo() uint32 { return OpGetClientInfo }

// IsReplyTo returns the command GetClientInfoListReply is the reply to.
func (*GetClientInfoListReply) IsReplyTo() uint32 { return OpGetClientInfoList }

// IsReplyTo returns the command GetSinkInputInfoReply is the reply to.
func (*GetSinkInputInfoReply) IsReplyTo() uint32 { return OpGetSinkInputInfo }

// IsReplyTo returns the command GetSinkInputInfoListReply is the reply to.
func (*GetSinkInputInfoListReply) IsReplyTo() uint32 { return OpGetSinkInputInfoList }

// IsReplyTo returns the command GetSourceOutputInfoReply is the reply to.
func (*GetSourceOutputInfoReply) IsReplyTo() uint32 { return OpGetSourceOutputInfo }

// IsReplyTo returns the command GetSourceOutputInfoListReply is the reply to.
func (*GetSourceOutputInfoListReply) IsReplyTo() uint32 { return OpGetSourceOutputInfoList }

// IsReplyTo returns the command GetSampleInfoReply is the reply to.
func (*GetSampleInfoReply) IsReplyTo() uint32 { return OpGetSampleInfo }

// IsReplyTo returns the command GetSampleInfoListReply is the reply to.
func (*GetSampleInfoListReply) IsReplyTo() uint32 { return OpGetSampleInfoList }

// IsReplyTo returns the command LoadModuleReply is the reply to.
func (*LoadModuleReply) IsReplyTo() uint32 { return OpLoadModule }

// IsReplyTo returns the command GetRecordLatencyReply is the reply to.
func (*GetRecordLatencyReply) IsReplyTo() uint32 { return OpGetRecordLatency }

// IsReplyTo returns the command SetPlaybackStreamBufferAttrReply is the reply to.
func (*SetPlaybackStreamBufferAttrReply) IsReplyTo() uint32 { return OpSetPlaybackStreamBufferAttr }

// IsReplyTo returns the command SetRecordStreamBufferAttrReply is the reply to.
func (*SetRecordStreamBufferAttrReply) IsReplyTo() uint32 { return OpSetRecordStreamBufferAttr }

// IsReplyTo returns the command GetCardInfoReply is the reply to.
func (*GetCardInfoReply) IsReplyTo() uint32 { return OpGetCardInfo }

// IsReplyTo returns the command GetCardInfoListReply is the reply to.
func (*GetCardInfoListReply) IsReplyTo() uint32 { return OpGetCardInfoList }

// SERVER -> CLIENT MESSAGES

// Request is the protocol message OpRequest, which the server sends and the client hands to its Callback.
type Request struct {
	StreamIndex uint32
	Length      uint32
}

// Overflow is the protocol message OpOverflow, which the server sends and the client hands to its Callback.
type Overflow struct {
	StreamIndex uint32
}

// Underflow is the protocol message OpUnderflow, which the server sends and the client hands to its Callback.
type Underflow struct {
	StreamIndex uint32
	Offset      int64 `pulse:"23"`
}

// PlaybackStreamKilled is the protocol message OpPlaybackStreamKilled, which the server sends and the client hands to its Callback.
type PlaybackStreamKilled struct{ StreamIndex uint32 }

// RecordStreamKilled is the protocol message OpRecordStreamKilled, which the server sends and the client hands to its Callback.
type RecordStreamKilled struct{ StreamIndex uint32 }

// SubscribeEvent is the protocol message OpSubscribeEvent, which the server sends and the client hands to its Callback.
type SubscribeEvent struct {
	Event SubscriptionEventType
	Index uint32
}

// PlaybackStreamSuspended is the protocol message OpPlaybackStreamSuspended, which the server sends and the client hands to its Callback.
type PlaybackStreamSuspended struct {
	StreamIndex uint32
	Suspended   bool
}

// RecordStreamSuspended is the protocol message OpRecordStreamSuspended, which the server sends and the client hands to its Callback.
type RecordStreamSuspended struct {
	StreamIndex uint32
	Suspended   bool
}

// PlaybackStreamMoved is the protocol message OpPlaybackStreamMoved, which the server sends and the client hands to its Callback.
type PlaybackStreamMoved struct {
	StreamIndex uint32
	DestIndex   uint32
	DestName    string
	Suspended   bool

	BufferMaxLength       uint32       `pulse:"13"`
	BufferTargetLength    uint32       `pulse:"13"`
	BufferPrebufferLength uint32       `pulse:"13"`
	BufferMinimumRequest  uint32       `pulse:"13"`
	SinkLatency           Microseconds `pulse:"13"`
}

// RecordStreamMoved is the protocol message OpRecordStreamMoved, which the server sends and the client hands to its Callback.
type RecordStreamMoved struct {
	StreamIndex uint32
	DestIndex   uint32
	DestName    string
	Suspended   bool

	BufferMaxLength uint32       `pulse:"13"`
	BufferFragSize  uint32       `pulse:"13"`
	SourceLatency   Microseconds `pulse:"13"`
}

// Started is the protocol message OpStarted, which the server sends and the client hands to its Callback.
type Started struct{ StreamIndex uint32 }

// ClientEvent is the protocol message OpClientEvent, which the server sends and the client hands to its Callback.
type ClientEvent struct {
	Event      string
	Properties PropList
}

// PlaybackStreamEvent is the protocol message OpPlaybackStreamEvent, which the server sends and the client hands to its Callback.
type PlaybackStreamEvent struct {
	StreamIndex uint32
	Event       string
	Properties  PropList
}

// RecordStreamEvent is the protocol message OpRecordStreamEvent, which the server sends and the client hands to its Callback.
type RecordStreamEvent struct {
	StreamIndex uint32
	Event       string
	Properties  PropList
}

// PlaybackBufferAttrChanged is the protocol message OpPlaybackBufferAttrChanged, which the server sends and the client hands to its Callback.
type PlaybackBufferAttrChanged struct {
	StreamIndex           uint32
	BufferMaxLength       uint32
	BufferTargetLength    uint32
	BufferPrebufferLength uint32
	BufferMinimumRequest  uint32
	SinkLatency           Microseconds
}

package constants

// Namespaces for Cast protocol.
const (
	NsConnection = "urn:x-cast:com.google.cast.tp.connection"
	NsHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	NsReceiver   = "urn:x-cast:com.google.cast.receiver"
	NsMedia      = "urn:x-cast:com.google.cast.media"
	NsDeviceAuth = "urn:x-cast:com.google.cast.tp.deviceauth"
)

// Message types.
const (
	TypeConnect        = "CONNECT"
	TypeClose          = "CLOSE"
	TypePing           = "PING"
	TypePong           = "PONG"
	TypeGetStatus      = "GET_STATUS"
	TypeReceiverStatus = "RECEIVER_STATUS"
	TypeSetVolume      = "SET_VOLUME"
	TypeMediaStatus    = "MEDIA_STATUS"
	TypePlay           = "PLAY"
	TypePause          = "PAUSE"
	TypeStop           = "STOP"
	TypeQueueUpdate    = "QUEUE_UPDATE"
	TypeSeek           = "SEEK"
	TypeLoad           = "LOAD"
	TypeLoadFailed     = "LOAD_FAILED"
)

// Application and session identifiers.
const (
	PlatformReceiverID    = "receiver-0"
	WildcardDestinationID = "*"
	DefaultMediaAppID     = "CC1AD845"
	AppTransportID        = "media-1"
	AppSessionID          = "1"
	MediaSessionID        = 1
	MetadataTypeMusic     = 3 // MusicTrackMediaMetadata
)

// SupportedMediaCommands bitmask flags.
const (
	CommandPause        = 1
	CommandSeek         = 2
	CommandStreamVolume = 4
	CommandStreamMute   = 8
	CommandQueueNext    = 64
	CommandQueuePrev    = 128

	SupportedCommands = CommandPause |
		CommandSeek |
		CommandStreamVolume |
		CommandStreamMute |
		CommandQueueNext |
		CommandQueuePrev
)

// Network defaults.
const (
	DefaultPort  = 8009
	DefaultModel = "Google Home"
	MDNSService  = "_googlecast._tcp"
	MDNSDomain   = "local."
)

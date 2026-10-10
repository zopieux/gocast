package gocast

// Config holds configuration parameters for the Cast receiver server.
type Config struct {
	FriendlyName string
	Port         int
	Model        string
}

// CommandHandlers defines callback hooks for media and receiver actions.
type CommandHandlers struct {
	OnPlay      func()
	OnPause     func()
	OnStop      func()
	OnNext      func()
	OnPrevious  func()
	OnSeek      func(position float64)
	OnSetVolume func(percent int)
	OnSetMute   func(muted bool)
}

// MediaStatus represents the current playback and track metadata.
type MediaStatus struct {
	PlayerState string  // "IDLE", "PLAYING", "PAUSED", "BUFFERING" (case-insensitive)
	Title       string
	Artist      string
	Album       string
	Duration    float64 // in seconds
	Position    float64 // in seconds
}

package eventbus

const (
	EventLLMChunk       EventType = "llm.chunk"
	EventLLMDone        EventType = "llm.done"
	EventTTSChunk       EventType = "tts.chunk"
	EventAudioChunk     EventType = "audio.chunk"
	EventAudioDone      EventType = "audio.done"
	EventTextChunk      EventType = "text.chunk"
	EventASRChunk       EventType = "asr.chunk"
	EventTitleGenerated EventType = "title.generated"
	EventUserMessage    EventType = "user.message"
)

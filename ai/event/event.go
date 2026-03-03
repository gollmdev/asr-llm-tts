package event

type EventType string

const (
	EventLLMChunk            EventType = "llm.chunk"
	EventLLMDone             EventType = "llm.done"
	EventTTSChunk            EventType = "tts.chunk"
	EventAudioChunk          EventType = "audio.chunk"
	EventAudioDone           EventType = "audio.done"
	EventTextChunk           EventType = "text.chunk"
	EventASRChunk            EventType = "asr.chunk"
	EventTitleGenerated      EventType = "title.generated"
	EventUserMessage         EventType = "user.message"
	EventLLMResponseComplete EventType = "llm.response.complete"
	EventLLMCitation         EventType = "llm.citation"
	EventLLMThoughtChain     EventType = "llm.thought_chain"
)

type Event struct {
	Type EventType
	Data any
}

package server

import (
	"testing"
	"time"
)

func Test_serializeMessage(t *testing.T) {
	// Test the serialization of a message
	message := &ServerMessage{
		BaseMessage: BaseMessage{
			Id:        1,
			Timestamp: Now(),
		},
		Response: &Response{
			ResponseCode: 200,
			Data:         "test data",
		},
	}

	// The json output should match the expected format
	// Note: The json timestamp format is RFC3339Nano, so we need to format it
	// as such in the expected string
	expected := `{"id":1,"timestamp":"` + message.Timestamp.Format(time.RFC3339Nano) + `","response":{"response_code":200,"data":"test data"}}`

	bytes, err := serializeMessage(message)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(bytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(bytes))
	}
}

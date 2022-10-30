package rmq

import (
	"testing"
	"time"
)

func TestConnection(t *testing.T) {
	client := NewClient("tcp://test.mosquitto.org:1883")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Disconnect()
	}()
}

func TestInvalidConnectionDetails(t *testing.T) {
	client := NewClient("tcp://localhost:1883").
		WithUsername("test").
		WithPassword("test")
	err := client.Connect()
	if err == nil {
		t.Fatal("error expected")
	}
	defer func() {
		client.Disconnect()
	}()
}

func TestSubscribe(t *testing.T) {
	client := NewClient("tcp://test.mosquitto.org:1883")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Disconnect()
	}()

	_, err = client.Subscribe(t.Name(), AtLeastOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Unsubscribe(t.Name())
	}()
}

func TestPublishWithoutListening(t *testing.T) {
	client := NewClient("tcp://test.mosquitto.org:1883").WithRetry(1)
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Disconnect()
	}()

	if err := client.Publish(t.Name(), []byte(t.Name()), AtLeastOnce); err == nil {
		t.Fatal("error expected")
	}
}

func TestPublishWithListening(t *testing.T) {
	client := NewClient("tcp://test.mosquitto.org:1883")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Disconnect()
	}()

	channel, err := client.Subscribe(t.Name(), AtLeastOnce)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		client.Unsubscribe(t.Name())
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := client.Publish(t.Name(), []byte(t.Name()), AtLeastOnce); err != nil {
			t.Error(err)
		}
	}()

	for range channel {
		break
	}
}

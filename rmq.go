package rmq

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type QoS byte

const (
	AtMostOnce  QoS = 0
	AtLeastOnce QoS = 1
	ExactlyOnce QoS = 2
)

type Client struct {
	url          string
	username     string
	password     string
	retries      int
	orderMatters bool
	mqttClient   mqtt.Client
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewClient(url string) *Client {
	return &Client{
		url:          url,
		retries:      3,
		orderMatters: false,
	}
}

func (client *Client) WithUsername(username string) *Client {
	client.username = username
	return client
}

func (client *Client) WithPassword(password string) *Client {
	client.password = password
	return client
}

func (client *Client) WithRetry(number int) *Client {
	client.retries = number
	return client
}

func (client *Client) WithOrderMatters() *Client {
	client.orderMatters = true
	return client
}

func (client *Client) Connect() error {
	options := mqtt.NewClientOptions()
	options.SetClientID(strconv.FormatInt(time.Now().UnixNano(), 36))
	options.SetKeepAlive(3 * time.Second)
	options.SetAutoAckDisabled(false)
	options.SetOrderMatters(client.orderMatters)
	options.AddBroker(client.url)

	if client.username != "" {
		options.SetUsername(client.username)
	}

	if client.password != "" {
		options.SetPassword(client.password)
	}

	client.mqttClient = mqtt.NewClient(options)

	token := client.mqttClient.Connect()
	token.Wait()

	if err := token.Error(); err != nil {
		return err
	}

	return nil
}

func (client *Client) Disconnect() {
	client.mqttClient.Disconnect(250)
}

func (client *Client) publish(id string, topic string, payload []byte, qos QoS) error {
	responseTopic := fmt.Sprintf("%s/r/%s", topic, id)
	responseChannel := make(chan struct{})

	data := []byte(fmt.Sprintf("%s#%s", id, string(payload)))

	callback := func(c mqtt.Client, m mqtt.Message) {
		go func() {
			if string(m.Payload()) == "received" {
				responseChannel <- struct{}{}
			}
		}()
	}

	token := client.mqttClient.Subscribe(responseTopic, byte(qos), callback)
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}

	defer func() {
		token := client.mqttClient.Unsubscribe(responseTopic)
		token.Wait()
	}()

	token = client.mqttClient.Publish(topic, byte(qos), false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}

	select {
	case <-responseChannel:
		return nil
	case <-time.After(time.Second):
		return fmt.Errorf("response timed out")
	}
}

func (client *Client) Publish(topic string, payload []byte, qos QoS) error {
	id := generateId(8)
	var err error

	for i := 0; i < client.retries; i++ {
		err = client.publish(id, topic, payload, qos)
		if err != nil {
			continue
		}

		return nil
	}

	return err
}

func (client *Client) Subscribe(topic string, qos QoS) (chan []byte, error) {
	channel := make(chan []byte)

	callback := func(c mqtt.Client, m mqtt.Message) {
		go func() {
			split := strings.Split(string(m.Payload()), "#")
			if len(split) != 2 {
				return
			}

			id := split[0]
			payload := []byte(split[1])
			responseTopic := fmt.Sprintf("%s/r/%s", topic, id)

			go func() {
				for i := 0; i < 3; i++ {
					token := client.mqttClient.Publish(responseTopic, byte(qos), false, "received")
					token.Wait()

					if err := token.Error(); err != nil {
						continue
					}
					return
				}
			}()

			channel <- payload
		}()
	}

	token := client.mqttClient.Subscribe(topic, byte(qos), callback)
	token.Wait()

	if err := token.Error(); err != nil {
		return nil, err
	}

	return channel, nil
}

func (client *Client) Unsubscribe(topic string) error {
	token := client.mqttClient.Unsubscribe(topic)
	token.Wait()
	return token.Error()
}

func generateId(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

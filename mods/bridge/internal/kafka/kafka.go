package kafka

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/machbase/neo-server/mods/logging"
)

type bridge struct {
	log      logging.Log
	name     string
	path     string
	client   sarama.Client
	alive    bool
	stopSig  chan bool
	stopWait sync.WaitGroup

	clientConf       *sarama.Config
	clientConnString []string

	consumers []sarama.Consumer
}

func New(name string, path string) *bridge {
	return &bridge{
		log:     logging.GetLog("kafka-bridge"),
		name:    name,
		path:    path,
		stopSig: make(chan bool),
	}
}

func (c *bridge) BeforeRegister() error {
	var err error
	c.clientConf = sarama.NewConfig()
	c.clientConf.ChannelBufferSize = 256 // default is 256
	c.clientConf.Consumer.Return.Errors = true
	c.clientConf.Consumer.Offsets.Initial = sarama.OffsetNewest

	fields := strings.Fields(c.path)
	for _, field := range fields {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "broker", "brokers", "server":
			toks := strings.Split(val, ",")
			c.clientConnString = append(c.clientConnString, toks...)
		case "channel-buffer-size":
			if size, err := strconv.ParseInt(val, 10, 32); err == nil {
				c.clientConf.ChannelBufferSize = int(size)
			} else {
				c.log.Warnf("bridge '%s' invalid value, %s=%s, %s", c.name, key, val, err.Error())
			}
		case "consumer-offsets-autocommit-enable":
			if flag, err := strconv.ParseBool(val); err == nil {
				c.clientConf.Consumer.Offsets.AutoCommit.Enable = flag
			} else {
				c.log.Warnf("bridge '%s' invalid value, %s=%s, %s", c.name, key, val, err.Error())
			}
		case "consumer-offsets-autocommit-interval":
			if dur, err := time.ParseDuration(val); err == nil {
				c.clientConf.Consumer.Offsets.AutoCommit.Interval = dur
			} else {
				c.log.Warnf("bridge '%s' invalid value, %s=%s, %s", c.name, key, val, err.Error())
			}
		default:
			c.log.Warnf("bridge '%s' unknown option, %s=%s", c.name, key, val)
		}
	}
	c.client, err = sarama.NewClient(c.clientConnString, c.clientConf)
	if err != nil {
		c.alive = true
	}
	return err
}

func (c *bridge) AfterUnregister() error {
	c.alive = false
	for range c.consumers {
		c.stopSig <- true
	}
	c.stopWait.Wait()
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (kafka)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) Subscribe(topic string) error {
	partitionIds, err := c.client.Partitions(topic)
	if err != nil {
		return err
	}

	lastOffsets := make([]int64, len(partitionIds))
	for i, partId := range partitionIds {
		offset, err := c.client.GetOffset(topic, partId, sarama.OffsetNewest)
		if err != nil {
			return err
		}
		lastOffsets[i] = offset
	}

	consumer, err := sarama.NewConsumerFromClient(c.client)
	if err != nil {
		return err
	}

	partitionConsumers := make([]sarama.PartitionConsumer, len(partitionIds))
	for i, partId := range partitionIds {
		partitionConsumer, err := consumer.ConsumePartition(topic, partId, lastOffsets[i])
		if err != nil {
			return err
		}
		partitionConsumers[i] = partitionConsumer
	}

	for _, pc := range partitionConsumers {
		partitionConsumer := pc
		go func() {
			for c.alive {
				select {
				case err := <-partitionConsumer.Errors():
					c.log.Warnf("Topic %s Partition %d Error %s", err.Topic, err.Partition, err.Err.Error())
				case msg := <-partitionConsumer.Messages():
					c.log.Tracef("Topic %s Consumed message offset %d, Partition %d", msg.Topic, msg.Offset, msg.Partition)
				}
			}
		}()
	}

	c.consumers = append(c.consumers, consumer)
	c.stopWait.Add(1)
	go func() {
		for c.alive {
			<-c.stopSig
		}
		for _, pc := range partitionConsumers {
			pc.Close()
		}
		consumer.Close()
		c.stopWait.Done()
	}()
	return nil
}

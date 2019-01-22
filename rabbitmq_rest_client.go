package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
)

// RabbitHTTPClient is a minimal client to the rabbitmq management REST api.
// It implements only functions needed by this tool (i.e. GET on some of the
// resources).  The messages structs were generated using json-to-go (
// https://mholt.github.io/json-to-go/ RabbitMQ HTTP API documentation can be).
type RabbitHTTPClient struct {
	uri    string
	client *http.Client
}

// NewRabbitHTTPClient returns a new instance of an RabbitHTTPClient
func NewRabbitHTTPClient(uri string,
	tlsConfig *tls.Config) *RabbitHTTPClient {
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: tr}
	return &RabbitHTTPClient{uri, client}
}

type httpRequest struct {
	uri string
	t   reflect.Type // type of expected result
}

type httpResponse struct {
	result interface{}
	err    error
}

// TODO split function in http and unmarshaling part
func (s RabbitHTTPClient) getResource(request httpRequest) chan httpResponse {
	res := make(chan httpResponse)
	go func() {
		r := reflect.New(request.t).Interface()
		resp, err := s.client.Get(request.uri)
		if err != nil {
			res <- httpResponse{result: r, err: err}
			return
		}

		if resp.StatusCode != 200 {
			err := errors.New(resp.Status)
			res <- httpResponse{result: r, err: err}
			return
		}

		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)

		err = json.Unmarshal(buf.Bytes(), r)
		if err != nil {
			res <- httpResponse{result: r, err: err}
			return
		}
		res <- httpResponse{result: r, err: nil}
	}()
	return res
}

func (s RabbitHTTPClient) delResource(uri string) error {
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return errors.New(resp.Status)
	}
	defer resp.Body.Close()
	return nil
}

// BrokerInfo represents the result of multiple RabbitMQ ressources
type BrokerInfo struct {
	Overview    RabbitOverview
	Connections []RabbitConnection
	Exchanges   []RabbitExchange
	Queues      []RabbitQueue
	Consumers   []RabbitConsumer
	Bindings    []RabbitBinding
}

func (s RabbitHTTPClient) getConnections() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/connections", reflect.TypeOf([]RabbitConnection{})})
}

func (s RabbitHTTPClient) getOverview() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/overview", reflect.TypeOf(RabbitOverview{})})
}

func (s RabbitHTTPClient) getExchanges() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/exchanges", reflect.TypeOf([]RabbitExchange{})})
}

func (s RabbitHTTPClient) getQueues() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/queues", reflect.TypeOf([]RabbitQueue{})})
}

func (s RabbitHTTPClient) getConsumers() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/consumers", reflect.TypeOf([]RabbitConsumer{})})
}

func (s RabbitHTTPClient) getBindings() chan httpResponse {
	return s.getResource(httpRequest{s.uri + "/bindings", reflect.TypeOf([]RabbitBinding{})})
}

// BrokerInfo gets all resources of the broker at a time, in parallel
// TODO reduce complexity
func (s RabbitHTTPClient) BrokerInfo() (r BrokerInfo, err error) {
	const numExpectedResources = 6
	count := 0

	// get...() returns channel which will provide result when available
	// TODO use Context
	overview := s.getOverview()
	connections := s.getConnections()
	exchanges := s.getExchanges()
	queues := s.getQueues()
	consumers := s.getConsumers()
	bindings := s.getBindings()

	for {
		var res httpResponse
		select {
		case res := <-overview:
			r.Overview = *(res.result.(*RabbitOverview))
		case res = <-connections:
			r.Connections = *(res.result.(*[]RabbitConnection))
		case res = <-exchanges:
			r.Exchanges = *(res.result.(*[]RabbitExchange))
		case res = <-queues:
			r.Queues = *(res.result.(*[]RabbitQueue))
		case res = <-consumers:
			r.Consumers = *(res.result.(*[]RabbitConsumer))
		case res = <-bindings:
			r.Bindings = *(res.result.(*[]RabbitBinding))
			// TODO add timeout
		}
		if res.err != nil {
			err = res.err
			return
		}
		count++
		if count == numExpectedResources {
			return
		}
	}
}

// Connections returns the /connections resource of the broker
func (s RabbitHTTPClient) Connections() ([]RabbitConnection, error) {
	result := <-s.getConnections()
	return *(result.result.(*[]RabbitConnection)), result.err
}

// Overview returns the /overview resource of the broker
func (s RabbitHTTPClient) Overview() (RabbitOverview, error) {
	result := <-s.getOverview()
	return *(result.result.(*RabbitOverview)), result.err
}

// Exchanges returns the /exchanges resource of the broker
func (s RabbitHTTPClient) Exchanges() ([]RabbitExchange, error) {
	result := <-s.getExchanges()
	return *(result.result.(*[]RabbitExchange)), result.err
}

// Queues returns the /queues resource of the broker
func (s RabbitHTTPClient) Queues() ([]RabbitQueue, error) {
	result := <-s.getQueues()
	return *(result.result.(*[]RabbitQueue)), result.err
}

// Consumers returns the /consumers resource of the broker
func (s RabbitHTTPClient) Consumers() ([]RabbitConsumer, error) {
	result := <-s.getConsumers()
	return *(result.result.(*[]RabbitConsumer)), result.err
}

// Bindings returns the /bindings resource of the broker
func (s RabbitHTTPClient) Bindings() ([]RabbitBinding, error) {
	result := <-s.getBindings()
	return *(result.result.(*[]RabbitBinding)), result.err
}

// Channels returns the /channels resource of the broker. TODO not yet used.
// func (s RabbitHTTPClient) Channels() ([]RabbitChannel, error) {
//     result := <-s.getResource2(httpRequest{s.uri + "/channels", reflect.TypeOf([]RabbitChannel{})})
//     return *(result.result.(*[]RabbitChannel)), result.err
// }

// CloseConnection closes a connection by DELETING the associated resource
func (s RabbitHTTPClient) CloseConnection(conn, reason string) error {
	return s.delResource(s.uri + "/connections/" + conn)
}

// RabbitConnection models the /connections resource of the rabbitmq http api
type RabbitConnection struct {
	ReductionsDetails struct {
		Rate float64 `json:"rate"`
	} `json:"reductions_details"`
	Reductions     int `json:"reductions"`
	RecvOctDetails struct {
		Rate float64 `json:"rate"`
	} `json:"recv_oct_details"`
	RecvOct        int `json:"recv_oct"`
	SendOctDetails struct {
		Rate float64 `json:"rate"`
	} `json:"send_oct_details"`
	SendOct          int   `json:"send_oct"`
	ConnectedAt      int64 `json:"connected_at"`
	ClientProperties struct {
		Product      string `json:"product"`
		Version      string `json:"version"`
		Capabilities struct {
			ConnectionBlocked    bool `json:"connection.blocked"`
			ConsumerCancelNotify bool `json:"consumer_cancel_notify"`
		} `json:"capabilities"`
	} `json:"client_properties"`
	ChannelMax        int         `json:"channel_max"`
	FrameMax          int         `json:"frame_max"`
	Timeout           int         `json:"timeout"`
	Vhost             string      `json:"vhost"`
	User              string      `json:"user"`
	Protocol          string      `json:"protocol"`
	SslHash           interface{} `json:"ssl_hash"`
	SslCipher         interface{} `json:"ssl_cipher"`
	SslKeyExchange    interface{} `json:"ssl_key_exchange"`
	SslProtocol       interface{} `json:"ssl_protocol"`
	AuthMechanism     string      `json:"auth_mechanism"`
	PeerCertValidity  interface{} `json:"peer_cert_validity"`
	PeerCertIssuer    interface{} `json:"peer_cert_issuer"`
	PeerCertSubject   interface{} `json:"peer_cert_subject"`
	Ssl               bool        `json:"ssl"`
	PeerHost          string      `json:"peer_host"`
	Host              string      `json:"host"`
	PeerPort          int         `json:"peer_port"`
	Port              int         `json:"port"`
	Name              string      `json:"name"`
	Node              string      `json:"node"`
	Type              string      `json:"type"`
	GarbageCollection struct {
		MinorGcs        int `json:"minor_gcs"`
		FullsweepAfter  int `json:"fullsweep_after"`
		MinHeapSize     int `json:"min_heap_size"`
		MinBinVheapSize int `json:"min_bin_vheap_size"`
		MaxHeapSize     int `json:"max_heap_size"`
	} `json:"garbage_collection"`
	Channels int    `json:"channels"`
	State    string `json:"state"`
	SendPend int    `json:"send_pend"`
	SendCnt  int    `json:"send_cnt"`
	RecvCnt  int    `json:"recv_cnt"`
}

// RabbitChannel models the /channels resource of the rabbitmq http api
type RabbitChannel struct {
	ReductionsDetails struct {
		Rate float64 `json:"rate"`
	} `json:"reductions_details"`
	Reductions   int `json:"reductions"`
	MessageStats struct {
		ReturnUnroutableDetails struct {
			Rate float64 `json:"rate"`
		} `json:"return_unroutable_details"`
		ReturnUnroutable int `json:"return_unroutable"`
		ConfirmDetails   struct {
			Rate float64 `json:"rate"`
		} `json:"confirm_details"`
		Confirm        int `json:"confirm"`
		PublishDetails struct {
			Rate float64 `json:"rate"`
		} `json:"publish_details"`
		Publish int `json:"publish"`
	} `json:"message_stats"`
	Vhost             string `json:"vhost"`
	User              string `json:"user"`
	Number            int    `json:"number"`
	Name              string `json:"name"`
	Node              string `json:"node"`
	ConnectionDetails struct {
		PeerHost string `json:"peer_host"`
		PeerPort int    `json:"peer_port"`
		Name     string `json:"name"`
	} `json:"connection_details"`
	GarbageCollection struct {
		MinorGcs        int `json:"minor_gcs"`
		FullsweepAfter  int `json:"fullsweep_after"`
		MinHeapSize     int `json:"min_heap_size"`
		MinBinVheapSize int `json:"min_bin_vheap_size"`
		MaxHeapSize     int `json:"max_heap_size"`
	} `json:"garbage_collection"`
	State                  string `json:"state"`
	GlobalPrefetchCount    int    `json:"global_prefetch_count"`
	PrefetchCount          int    `json:"prefetch_count"`
	AcksUncommitted        int    `json:"acks_uncommitted"`
	MessagesUncommitted    int    `json:"messages_uncommitted"`
	MessagesUnconfirmed    int    `json:"messages_unconfirmed"`
	MessagesUnacknowledged int    `json:"messages_unacknowledged"`
	ConsumerCount          int    `json:"consumer_count"`
	Confirm                bool   `json:"confirm"`
	Transactional          bool   `json:"transactional"`
}

// RabbitOverview models the /overview resource of the rabbitmq http api
type RabbitOverview struct {
	ManagementVersion string `json:"management_version"`
	RatesMode         string `json:"rates_mode"`
	ExchangeTypes     []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	} `json:"exchange_types"`
	RabbitmqVersion   string `json:"rabbitmq_version"`
	ClusterName       string `json:"cluster_name"`
	ErlangVersion     string `json:"erlang_version"`
	ErlangFullVersion string `json:"erlang_full_version"`
	MessageStats      struct {
		DiskReads        int `json:"disk_reads"`
		DiskReadsDetails struct {
			Rate float64 `json:"rate"`
		} `json:"disk_reads_details"`
		DiskWrites        int `json:"disk_writes"`
		DiskWritesDetails struct {
			Rate float64 `json:"rate"`
		} `json:"disk_writes_details"`
	} `json:"message_stats"`
	QueueTotals struct {
		MessagesReady        int `json:"messages_ready"`
		MessagesReadyDetails struct {
			Rate float64 `json:"rate"`
		} `json:"messages_ready_details"`
		MessagesUnacknowledged        int `json:"messages_unacknowledged"`
		MessagesUnacknowledgedDetails struct {
			Rate float64 `json:"rate"`
		} `json:"messages_unacknowledged_details"`
		Messages        int `json:"messages"`
		MessagesDetails struct {
			Rate float64 `json:"rate"`
		} `json:"messages_details"`
	} `json:"queue_totals"`
	ObjectTotals struct {
		Consumers   int `json:"consumers"`
		Queues      int `json:"queues"`
		Exchanges   int `json:"exchanges"`
		Connections int `json:"connections"`
		Channels    int `json:"channels"`
	} `json:"object_totals"`
	StatisticsDbEventQueue int    `json:"statistics_db_event_queue"`
	Node                   string `json:"node"`
	Listeners              []struct {
		Node      string `json:"node"`
		Protocol  string `json:"protocol"`
		IPAddress string `json:"ip_address"`
		Port      int    `json:"port"`
		// workaround for rabbitmq returning empty array OR valid object
		// here. TODO check / further investigate.-
		/*Dummy      []interface{} `json:"socket_opts,omitempty"`
		SocketOpts struct {
			Backlog int  `json:"backlog"`
			Nodelay bool `json:"nodelay"`
			//Linger      []interface{} `json:"linger"`
			ExitOnClose bool `json:"exit_on_close"`
		} `json:"socket_opts"`*/
	} `json:"listeners"`
	Contexts []struct {
		Node        string `json:"node"`
		Description string `json:"description"`
		Path        string `json:"path"`
		Port        string `json:"port"`
		Ssl         string `json:"ssl"`
	} `json:"contexts"`
}

// RabbitQueue models the /queues resource of the rabbitmq http api
type RabbitQueue struct {
	MessagesDetails struct {
		Rate float64 `json:"rate"`
	} `json:"messages_details"`
	Messages                      int `json:"messages"`
	MessagesUnacknowledgedDetails struct {
		Rate float64 `json:"rate"`
	} `json:"messages_unacknowledged_details"`
	MessagesUnacknowledged int `json:"messages_unacknowledged"`
	MessagesReadyDetails   struct {
		Rate float64 `json:"rate"`
	} `json:"messages_ready_details"`
	MessagesReady     int `json:"messages_ready"`
	ReductionsDetails struct {
		Rate float64 `json:"rate"`
	} `json:"reductions_details"`
	Reductions int    `json:"reductions"`
	Node       string `json:"node"`
	Arguments  struct {
	} `json:"arguments"`
	Exclusive            bool   `json:"exclusive"`
	AutoDelete           bool   `json:"auto_delete"`
	Durable              bool   `json:"durable"`
	Vhost                string `json:"vhost"`
	Name                 string `json:"name"`
	MessageBytesPagedOut int    `json:"message_bytes_paged_out"`
	MessagesPagedOut     int    `json:"messages_paged_out"`
	BackingQueueStatus   struct {
		Mode string `json:"mode"`
		Q1   int    `json:"q1"`
		Q2   int    `json:"q2"`
		//		Delta             []interface{} `json:"delta"`
		Q3  int `json:"q3"`
		Q4  int `json:"q4"`
		Len int `json:"len"`
		//		TargetRAMCount    int     `json:"target_ram_count"`	// string or int -> need further research here when attr is in need ("infinity")
		NextSeqID         int     `json:"next_seq_id"`
		AvgIngressRate    float64 `json:"avg_ingress_rate"`
		AvgEgressRate     float64 `json:"avg_egress_rate"`
		AvgAckIngressRate float64 `json:"avg_ack_ingress_rate"`
		AvgAckEgressRate  float64 `json:"avg_ack_egress_rate"`
	} `json:"backing_queue_status"`
	//	HeadMessageTimestamp       interface{} `json:"head_message_timestamp"`
	MessageBytesPersistent     int `json:"message_bytes_persistent"`
	MessageBytesRAM            int `json:"message_bytes_ram"`
	MessageBytesUnacknowledged int `json:"message_bytes_unacknowledged"`
	MessageBytesReady          int `json:"message_bytes_ready"`
	MessageBytes               int `json:"message_bytes"`
	MessagesPersistent         int `json:"messages_persistent"`
	MessagesUnacknowledgedRAM  int `json:"messages_unacknowledged_ram"`
	MessagesReadyRAM           int `json:"messages_ready_ram"`
	MessagesRAM                int `json:"messages_ram"`
	GarbageCollection          struct {
		MinorGcs        int `json:"minor_gcs"`
		FullsweepAfter  int `json:"fullsweep_after"`
		MinHeapSize     int `json:"min_heap_size"`
		MinBinVheapSize int `json:"min_bin_vheap_size"`
		MaxHeapSize     int `json:"max_heap_size"`
	} `json:"garbage_collection"`
	State string `json:"state"`
	//	RecoverableSlaves    interface{} `json:"recoverable_slaves"`
	Consumers int `json:"consumers"`
	//	ExclusiveConsumerTag interface{} `json:"exclusive_consumer_tag"`
	//	Policy               interface{} `json:"policy"`
	//	ConsumerUtilisation  interface{} `json:"consumer_utilisation"`
	// TODO use cusom marshaller and parese into time.Time
	IdleSince string `json:"idle_since"`
	Memory    int    `json:"memory"`
}

// RabbitBinding models the /bindings resource of the rabbitmq http api
type RabbitBinding struct {
	Source          string `json:"source"`
	Vhost           string `json:"vhost"`
	Destination     string `json:"destination"`
	DestinationType string `json:"destination_type"`
	RoutingKey      string `json:"routing_key"`
	//Arguments       struct {
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	//} `json:"arguments"`
	PropertiesKey string `json:"properties_key"`
}

// RabbitExchange models the /exchanges resource of the rabbitmq http api
type RabbitExchange struct {
	Name       string                 `json:"name"`
	Vhost      string                 `json:"vhost"`
	Type       string                 `json:"type"`
	Durable    bool                   `json:"durable"`
	AutoDelete bool                   `json:"auto_delete"`
	Internal   bool                   `json:"internal"`
	Arguments  map[string]interface{} `json:"arguments,omitempty"`
	//Arguments  struct {
	//} `json:"arguments"`
	MessageStats struct {
		PublishOut        int `json:"publish_out"`
		PublishOutDetails struct {
			Rate float64 `json:"rate"`
		} `json:"publish_out_details"`
		PublishIn        int `json:"publish_in"`
		PublishInDetails struct {
			Rate float64 `json:"rate"`
		} `json:"publish_in_details"`
	} `json:"message_stats,omitempty"`
}

// ChannelDetails model channel_details in RabbitConsumer
type ChannelDetails struct {
	PeerHost       string `json:"peer_host"`
	PeerPort       int    `json:"peer_port"`
	ConnectionName string `json:"connection_name"`
	User           string `json:"user"`
	Number         int    `json:"number"`
	Node           string `json:"node"`
	Name           string `json:"name"`
}

// UnmarshalJSON is a custom unmarshaler as a WORKAROUND for RabbitMQ API
// returning "[]" instead of null.  To make sure deserialization does not
// break, we catch this case, and return an empty ChannelDetails struct.
// see e.g. https://github.com/rabbitmq/rabbitmq-management/issues/424
func (d *ChannelDetails) UnmarshalJSON(data []byte) error {
	// akias ChannelDetails to avoid recursion when callung Unmarshal
	type Alias ChannelDetails
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if data[0] == '[' {
		// JSON array detected
		return nil
	}
	return json.Unmarshal(data, aux)
}

// RabbitConsumer models the /consumers resource of the rabbitmq http api
type RabbitConsumer struct {
	//	Arguments      []interface{} `json:"arguments"`
	PrefetchCount int    `json:"prefetch_count"`
	AckRequired   bool   `json:"ack_required"`
	Exclusive     bool   `json:"exclusive"`
	ConsumerTag   string `json:"consumer_tag"`
	// see WORKAROUND above
	ChannelDetails ChannelDetails `json:"channel_details,omitempty"`
	Queue          struct {
		Vhost string `json:"vhost"`
		Name  string `json:"name"`
	} `json:"queue"`
}

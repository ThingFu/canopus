package canopus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"sort"
	"strconv"
	"strings"
)

// Instantiates a new message object
// messageType (e.g. Confirm/Non-Confirm)
// CoAP code	404 - Not found etc
// Message ID	uint16 unique id
func NewMessage(messageType uint8, code CoapCode, messageId uint16) *Message {
	return &Message{
		MessageType: messageType,
		MessageId:   messageId,
		Code:        code,
	}
}

// Instantiates an empty message with a given message id
func NewEmptyMessage(id uint16) *Message {
	msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, id)

	return msg
}

// Instantiates an empty message of a specific type and message id
func NewMessageOfType(t uint8, id uint16) *Message {
	return &Message{
		MessageType: t,
		MessageId:   id,
	}
}

/*
     0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |Ver| T |  TKL  |      Code     |          Message ID           |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |   Token (if any, TKL bytes) ...
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |   Options (if any) ...
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |1 1 1 1 1 1 1 1|    Payload (if any) ...
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/

// Converts an array of bytes to a Mesasge object.
// An error is returned if a parsing error occurs
func BytesToMessage(data []byte) (*Message, error) {
	msg := &Message{}

	dataLen := len(data)
	if dataLen < 4 {
		return msg, ERR_PACKET_LENGTH_LESS_THAN_4
	}

	ver := data[DATA_HEADER] >> 6
	if ver != 1 {
		return nil, ERR_INVALID_VERSION
	}

	msg.MessageType = data[DATA_HEADER] >> 4 & 0x03
	tokenLength := data[DATA_HEADER] & 0x0f
	msg.Code = CoapCode(data[DATA_CODE])

	msg.MessageId = binary.BigEndian.Uint16(data[DATA_MSGID_START:DATA_MSGID_END])

	// Token
	if tokenLength > 0 {
		msg.Token = make([]byte, tokenLength)
		token := data[DATA_TOKEN_START : DATA_TOKEN_START+tokenLength]
		copy(msg.Token, token)
	}

	/*
	    0   1   2   3   4   5   6   7
	   +---------------+---------------+
	   |               |               |
	   |  Option Delta | Option Length |   1 byte
	   |               |               |
	   +---------------+---------------+
	   \                               \
	   /         Option Delta          /   0-2 bytes
	   \          (extended)           \
	   +-------------------------------+
	   \                               \
	   /         Option Length         /   0-2 bytes
	   \          (extended)           \
	   +-------------------------------+
	   \                               \
	   /                               /
	   \                               \
	   /         Option Value          /   0 or more bytes
	   \                               \
	   /                               /
	   \                               \
	   +-------------------------------+
	*/
	tmp := data[DATA_TOKEN_START+msg.GetTokenLength():]

	lastOptionId := 0
	for len(tmp) > 0 {
		if tmp[0] == PAYLOAD_MARKER {
			tmp = tmp[1:]
			break
		}

		optionDelta := int(tmp[0] >> 4)
		optionLength := int(tmp[0] & 0x0f)

		tmp = tmp[1:]
		switch optionDelta {
		case 13:
			optionDeltaExtended := int(tmp[0])
			optionDelta += optionDeltaExtended
			tmp = tmp[1:]
			break

		case 14:
			optionDeltaExtended := decodeInt(tmp[:1])
			optionDelta += int(optionDeltaExtended - uint32(269))
			tmp = tmp[2:]
			break

		case 15:
			return msg, ERR_OPTION_DELTA_USES_VALUE_15
		}
		lastOptionId += optionDelta

		switch optionLength {
		case 13:
			optionLengthExtended := int(tmp[0])
			optionLength += optionLengthExtended
			tmp = tmp[1:]
			break

		case 14:
			optionLengthExtended := decodeInt(tmp[:1])
			optionLength += int(optionLengthExtended - uint32(269))
			tmp = tmp[2:]
			break

		case 15:
			return msg, ERR_OPTION_LENGTH_USES_VALUE_15
		}

		optCode := OptionCode(lastOptionId)
		if optionLength > 0 {
			optionValue := tmp[:optionLength]

			switch optCode {
			case OPTION_URI_PORT, OPTION_CONTENT_FORMAT, OPTION_MAX_AGE, OPTION_ACCEPT, OPTION_SIZE1,
				OPTION_BLOCK1, OPTION_BLOCK2:
				msg.Options = append(msg.Options, NewOption(optCode, decodeInt(optionValue)))
				break

			case OPTION_URI_HOST, OPTION_LOCATION_PATH, OPTION_URI_PATH, OPTION_URI_QUERY,
				OPTION_LOCATION_QUERY, OPTION_PROXY_URI, OPTION_PROXY_SCHEME, OPTION_OBSERVE:
				msg.Options = append(msg.Options, NewOption(optCode, string(optionValue)))
				break

			default:
				if lastOptionId&0x01 == 1 {
					log.Println("Unknown Critical Option id " + strconv.Itoa(lastOptionId))
					return msg, ERR_UNKNOWN_CRITICAL_OPTION
				} else {
					log.Println("Unknown Option id " + strconv.Itoa(lastOptionId))
				}
				break
			}
			tmp = tmp[optionLength:]
		} else {
			msg.Options = append(msg.Options, NewOption(optCode, nil))
		}
	}
	msg.Payload = NewBytesPayload(tmp)
	err := ValidateMessage(msg)

	return msg, err
}

// type to sort the coap options list (which is mandatory) prior to transmission
type SortOptions []*Option

func (opts SortOptions) Len() int {
	return len(opts)
}

func (opts SortOptions) Swap(i, j int) {
	opts[i], opts[j] = opts[j], opts[i]
}

func (opts SortOptions) Less(i, j int) bool {
	return opts[i].Code < opts[j].Code
}

// Converts a message object to a byte array. Typically done prior to transmission
func MessageToBytes(msg *Message) ([]byte, error) {
	messageId := []byte{0, 0}
	binary.BigEndian.PutUint16(messageId, msg.MessageId)

	buf := bytes.Buffer{}
	buf.Write([]byte{(1 << 6) | (msg.MessageType << 4) | 0x0f&msg.GetTokenLength()})
	buf.Write([]byte{byte(msg.Code)})
	buf.Write([]byte{messageId[0]})
	buf.Write([]byte{messageId[1]})
	buf.Write(msg.Token)

	// Sort Options
	sort.Sort(SortOptions(msg.Options))

	lastOptionCode := 0
	for _, opt := range msg.Options {
		optCode := int(opt.Code)
		optDelta := optCode - lastOptionCode
		optDeltaValue, _ := getOptionHeaderValue(optDelta)

		byteValue := valueToBytes(opt.Value)
		valueLength := len(byteValue)
		optLength := valueLength
		optLengthValue, _ := getOptionHeaderValue(optLength)

		buf.Write([]byte{byte(optDeltaValue<<4 | optLengthValue)})

		if optDeltaValue == 13 {
			buf.Write([]byte{byte(optDelta - 13)})
		} else if optDeltaValue == 14 {
			tmpBuf := new(bytes.Buffer)
			binary.Write(tmpBuf, binary.BigEndian, uint16(optDelta-269))
			buf.Write(tmpBuf.Bytes())
		}

		if optLengthValue == 13 {
			buf.Write([]byte{byte(optLength - 13)})
		} else if optLengthValue == 14 {
			tmpBuf := new(bytes.Buffer)
			binary.Write(tmpBuf, binary.BigEndian, uint16(optLength-269))
			buf.Write(tmpBuf.Bytes())
		}

		buf.Write(byteValue)
		lastOptionCode = int(optCode)
	}

	if msg.Payload != nil {
		if msg.Payload.Length() > 0 {
			buf.Write([]byte{PAYLOAD_MARKER})
		}
		buf.Write(msg.Payload.GetBytes())
	}
	return buf.Bytes(), nil
}

func getOptionHeaderValue(optValue int) (int, error) {
	switch true {
	case optValue <= 12:
		return optValue, nil
		break

	case optValue <= 268:
		return 13, nil
		break

	case optValue <= 65804:
		return 14, nil
		break
	}
	return 0, errors.New("Invalid Option Delta")
}

// Validates a message object and returns any error upon validation failure
func ValidateMessage(msg *Message) error {
	if msg.MessageType > 3 {
		return ERR_UNKNOWN_MESSAGE_TYPE
	}

	if msg.GetTokenLength() > 8 {
		return ERR_INVALID_TOKEN_LENGTH
	}

	// Repeated Unrecognized Options
	for _, opt := range msg.Options {
		opts := msg.GetOptions(opt.Code)

		if len(opts) > 1 {
			if !IsRepeatableOption(opts[0]) {
				if opts[0].Code&0x01 == 1 {
					return ERR_UNKNOWN_CRITICAL_OPTION
				}
			}
		}
	}

	return nil
}

// A Message object represents a CoAP payload
type Message struct {
	MessageType uint8
	Code        CoapCode
	MessageId   uint16
	Payload     MessagePayload
	Token       []byte
	Options     []*Option
}

func (c Message) GetCodeString() string {
	codeClass := string(c.Code >> 5)
	codeDetail := string(c.Code & 0x1f)

	return codeClass + "." + codeDetail
}

func (c Message) GetMethod() uint8 {
	return (byte(c.Code) & 0x1f)
}

func (c Message) GetTokenLength() uint8 {
	return uint8(len(c.Token))
}

// Returns an array of options given an option code
func (c Message) GetOptions(id OptionCode) []*Option {
	var opts []*Option
	for _, val := range c.Options {
		if val.Code == id {
			opts = append(opts, val)
		}
	}
	return opts
}

// Returns the first option found for a given option code
func (c Message) GetOption(id OptionCode) *Option {
	for _, val := range c.Options {
		if val.Code == id {
			return val
		}
	}
	return nil
}

// Attempts to return the string value of an Option
func (c Message) GetOptionsAsString(id OptionCode) []string {
	opts := c.GetOptions(id)

	var str []string
	for _, o := range opts {
		if o.Value != nil {
			str = append(str, o.Value.(string))
		}
	}
	return str
}

// Returns the string value of the Location Path Options by joining and defining a / separator
func (c *Message) GetLocationPath() string {
	opts := c.GetOptionsAsString(OPTION_LOCATION_PATH)

	return strings.Join(opts, "/")
}

// Returns the string value of the Uri Path Options by joining and defining a / separator
func (c Message) GetUriPath() string {
	opts := c.GetOptionsAsString(OPTION_URI_PATH)

	return "/" + strings.Join(opts, "/")
}

// Add an Option to the message. If an option is not repeatable, it will replace
// any existing defined Option of the same type
func (m *Message) AddOption(code OptionCode, value interface{}) {
	opt := NewOption(code, value)
	if IsRepeatableOption(opt) {
		m.Options = append(m.Options, opt)
	} else {
		m.RemoveOptions(code)
		m.Options = append(m.Options, opt)
	}
}

// Add an array of Options to the message. If an option is not repeatable, it will replace
// any existing defined Option of the same type
func (m *Message) AddOptions(opts []*Option) {
	for _, opt := range opts {
		if IsRepeatableOption(opt) {
			m.Options = append(m.Options, opt)
		} else {
			m.RemoveOptions(opt.Code)
			m.Options = append(m.Options, opt)
		}
	}
}

// Copies the given list of options from another message to this one
func (m *Message) CloneOptions(cm *Message, opts ...OptionCode) {
	for _, opt := range opts {
		m.AddOptions(cm.GetOptions(opt))
	}
}

// Removes an Option
func (m *Message) RemoveOptions(id OptionCode) {
	var opts []*Option
	for _, opt := range m.Options {
		if opt.Code != id {
			opts = append(opts, opt)
		}
	}
	m.Options = opts
}

// Adds a string payload
func (m *Message) SetStringPayload(s string) {
	m.Payload = NewPlainTextPayload(s)
}

/* Helpers */
// Determines if a message contains options for proxying (i.e. Proxy-Scheme or Proxy-Uri)
func IsProxyRequest(msg *Message) bool {
	if msg.GetOption(OPTION_PROXY_SCHEME) != nil || msg.GetOption(OPTION_PROXY_URI) != nil {
		return true
	}
	return false
}

func valueToBytes(value interface{}) []byte {
	var v uint32

	switch i := value.(type) {
	case string:
		return []byte(i)
	case []byte:
		return i
	case MediaType:
		v = uint32(i)
	case byte:
		v = uint32(i)
	case int:
		v = uint32(i)
	case int32:
		v = uint32(i)
	case uint:
		v = uint32(i)
	case uint32:
		v = i
	default:
		break
	}

	return encodeInt(v)
}

// Returns the string value for a Message Payload
func PayloadAsString(p MessagePayload) string {
	if p == nil {
		return ""
	} else {
		return p.String()
	}
}

func decodeInt(b []byte) uint32 {
	tmp := []byte{0, 0, 0, 0}
	copy(tmp[4-len(b):], b)

	return binary.BigEndian.Uint32(tmp)
}

func encodeInt(v uint32) []byte {
	switch {
	case v == 0:
		return nil

	case v < 256:
		return []byte{byte(v)}

	case v < 65536:
		rv := []byte{0, 0}
		binary.BigEndian.PutUint16(rv, uint16(v))
		return rv

	case v < 16777216:
		rv := []byte{0, 0, 0, 0}
		binary.BigEndian.PutUint32(rv, uint32(v))
		return rv[1:]

	default:
		rv := []byte{0, 0, 0, 0}
		binary.BigEndian.PutUint32(rv, uint32(v))
		return rv
	}
}

// Determines if a message contains URI targetting a CoAP resource
func IsCoapUri(msg *Message) bool {
	uri := msg.GetUriPath()

	if strings.HasPrefix(uri, "coap") || strings.HasPrefix(uri, "coaps") {
		return true
	}
	return false
}

// Determines if a message contains URI targetting an HTTP resource
func IsHttpUri(uri string) bool {
	if strings.HasPrefix(uri, "http") || strings.HasPrefix(uri, "http") {
		return true
	}
	return false

}

// Gets the string representation of a CoAP Method code (e.g. GET, PUT, DELETE etc)
func MethodString(c CoapCode) string {
	switch c {
	case GET:
		return "GET"
		break

	case DELETE:
		return "DELETE"
		break

	case POST:
		return "POST"
		break

	case PUT:
		return "PUT"
		break
	}
	return ""
}

// Response Code Messages
// Creates a Non-Confirmable Empty Message
func EmptyMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_0_EMPTY, messageId)
}

// Creates a Non-Confirmable with CoAP Code 201 - Created
func CreatedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_201_CREATED, messageId)
}

// // Creates a Non-Confirmable with CoAP Code 202 - Deleted
func DeletedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_202_DELETED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 203 - Valid
func ValidMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_203_VALID, messageId)
}

// Creates a Non-Confirmable with CoAP Code 204 - Changed
func ChangedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_204_CHANGED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 205 - Content
func ContentMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_205_CONTENT, messageId)
}

// Creates a Non-Confirmable with CoAP Code 400 - Bad Request
func BadRequestMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_400_BAD_REQUEST, messageId)
}

// Creates a Non-Confirmable with CoAP Code 401 - Unauthorized
func UnauthorizedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_401_UNAUTHORIZED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 402 - Bad Option
func BadOptionMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_402_BAD_OPTION, messageId)
}

// Creates a Non-Confirmable with CoAP Code 403 - Forbidden
func ForbiddenMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_403_FORBIDDEN, messageId)
}

// Creates a Non-Confirmable with CoAP Code 404 - Not Found
func NotFoundMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_404_NOT_FOUND, messageId)
}

// Creates a Non-Confirmable with CoAP Code 405 - Method Not Allowed
func MethodNotAllowedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_405_METHOD_NOT_ALLOWED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 406 - Not Acceptable
func NotAcceptableMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_406_NOT_ACCEPTABLE, messageId)
}

// Creates a Non-Confirmable with CoAP Code 409 - Conflict
func ConflictMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_409_CONFLICT, messageId)
}

// Creates a Non-Confirmable with CoAP Code 412 - Precondition Failed
func PreconditionFailedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_412_PRECONDITION_FAILED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 413 - Request Entity Too Large
func RequestEntityTooLargeMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_413_REQUEST_ENTITY_TOO_LARGE, messageId)
}

// Creates a Non-Confirmable with CoAP Code 415 - Unsupported Content Format
func UnsupportedContentFormatMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_415_UNSUPPORTED_CONTENT_FORMAT, messageId)
}

// Creates a Non-Confirmable with CoAP Code 500 - Internal Server Error
func InternalServerErrorMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_500_INTERNAL_SERVER_ERROR, messageId)
}

// Creates a Non-Confirmable with CoAP Code 501 - Not Implemented
func NotImplementedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_501_NOT_IMPLEMENTED, messageId)
}

// Creates a Non-Confirmable with CoAP Code 502 - Bad Gateway
func BadGatewayMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_502_BAD_GATEWAY, messageId)
}

// Creates a Non-Confirmable with CoAP Code 503 - Service Unavailable
func ServiceUnavailableMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_503_SERVICE_UNAVAILABLE, messageId)
}

// Creates a Non-Confirmable with CoAP Code 504 - Gateway Timeout
func GatewayTimeoutMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_504_GATEWAY_TIMEOUT, messageId)
}

// Creates a Non-Confirmable with CoAP Code 505 - Proxying Not Supported
func ProxyingNotSupportedMessage(messageId uint16) *Message {
	return NewMessage(TYPE_NONCONFIRMABLE, COAPCODE_505_PROXYING_NOT_SUPPORTED, messageId)
}

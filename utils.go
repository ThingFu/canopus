package canopus

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// Generate a uint16 Message ID
func GenerateMessageId() uint16 {
	if MESSAGEID_CURR != 65535 {
		MESSAGEID_CURR++
	} else {
		MESSAGEID_CURR = 1
	}
	return uint16(MESSAGEID_CURR)
}

var genChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

// Generates a random token by a given length
func GenerateToken(l int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	token := make([]rune, l)
	for i := range token {
		token[i] = genChars[rand.Intn(len(genChars))]
	}
	return string(token)
}

// Converts to CoRE Resources Object from a CoRE String
func CoreResourcesFromString(str string) []*CoreResource {
	var re = regexp.MustCompile(`(<[^>]+>\s*(;\s*\w+\s*(=\s*(\w+|"([^"\\]*(\\.[^"\\]*)*)")\s*)?)*)`)
	var elemRe = regexp.MustCompile(`<[^>]*>`)

	var resources []*CoreResource
	m := re.FindAllString(str, -1)

	for _, match := range m {
		elemMatch := elemRe.FindString(match)
		target := elemMatch[1 : len(elemMatch)-1]

		resource := NewCoreResource()
		resource.Target = target

		if len(match) > len(elemMatch) {
			attrs := strings.Split(match[len(elemMatch)+1:], ";")

			for _, attr := range attrs {
				pair := strings.Split(attr, "=")

				resource.AddAttribute(pair[0], strings.Replace(pair[1], "\"", "", -1))
			}
		}
		resources = append(resources, resource)
	}
	return resources
}

func CoapCodeToString(code CoapCode) string {
	switch code {
	case GET:
		return "GET"

	case POST:
		return "POST"

	case PUT:
		return "PUT"

	case DELETE:
		return "DELETE"

	case COAPCODE_0_EMPTY:
		return "0 Empty"

	case COAPCODE_201_CREATED:
		return "201 Created"

	case COAPCODE_202_DELETED:
		return "202 Deleted"

	case COAPCODE_203_VALID:
		return "203 Valid"

	case COAPCODE_204_CHANGED:
		return "204 Changed"

	case COAPCODE_205_CONTENT:
		return "205 Content"

	case COAPCODE_400_BAD_REQUEST:
		return "400 Bad Request"

	case COAPCODE_401_UNAUTHORIZED:
		return "401 Unauthorized"

	case COAPCODE_402_BAD_OPTION:
		return "402 Bad Option"

	case COAPCODE_403_FORBIDDEN:
		return "403 Forbidden"

	case COAPCODE_404_NOT_FOUND:
		return "404 Not Found"

	case COAPCODE_405_METHOD_NOT_ALLOWED:
		return "405 Method Not Allowed"

	case COAPCODE_406_NOT_ACCEPTABLE:
		return "406 Not Acceptable"

	case COAPCODE_412_PRECONDITION_FAILED:
		return "412 Precondition Failed"

	case COAPCODE_413_REQUEST_ENTITY_TOO_LARGE:
		return "413 Request Entity Too Large"

	case COAPCODE_415_UNSUPPORTED_CONTENT_FORMAT:
		return "415 Unsupported Content Format"

	case COAPCODE_500_INTERNAL_SERVER_ERROR:
		return "500 Internal Server Error"

	case COAPCODE_501_NOT_IMPLEMENTED:
		return "501 Not Implemented"

	case COAPCODE_502_BAD_GATEWAY:
		return "502 Bad Gateway"

	case COAPCODE_503_SERVICE_UNAVAILABLE:
		return "503 Service Unavailable"

	case COAPCODE_504_GATEWAY_TIMEOUT:
		return "504 Gateway Timeout"

	case COAPCODE_505_PROXYING_NOT_SUPPORTED:
		return "505 Proxying Not Supported"

	default:
		return "Unknown"
	}
}

func ValidCoapMediaTypeCode(mt MediaType) bool {
	switch mt {
	case MEDIATYPE_TEXT_PLAIN, MEDIATYPE_TEXT_XML, MEDIATYPE_TEXT_CSV, MEDIATYPE_TEXT_HTML, MEDIATYPE_IMAGE_GIF,
		MEDIATYPE_IMAGE_JPEG, MEDIATYPE_IMAGE_PNG, MEDIATYPE_IMAGE_TIFF, MEDIATYPE_AUDIO_RAW, MEDIATYPE_VIDEO_RAW,
		MEDIATYPE_APPLICATION_LINK_FORMAT, MEDIATYPE_APPLICATION_XML, MEDIATYPE_APPLICATION_OCTET_STREAM, MEDIATYPE_APPLICATION_RDFXML,
		MEDIATYPE_APPLICATION_SOAPXML, MEDIATYPE_APPLICATION_ATOMXML, MEDIATYPE_APPLICATION_XMPPXML, MEDIATYPE_APPLICATION_EXI,
		MEDIATYPE_APPLICATION_FASTINFOSET, MEDIATYPE_APPLICATION_SOAPFASTINFOSET, MEDIATYPE_APPLICATION_JSON,
		MEDIATYPE_APPLICATION_X_OBIT_BINARY, MEDIATYPE_TEXT_PLAIN_VND_OMA_LWM2M, MEDIATYPE_TLV_VND_OMA_LWM2M,
		MEDIATYPE_JSON_VND_OMA_LWM2M, MEDIATYPE_OPAQUE_VND_OMA_LWM2M:
		return true
	}

	return false
}

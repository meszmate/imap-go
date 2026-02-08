package client

// ResponseHandler handles custom untagged responses.
type ResponseHandler func(name string, data string)

// ResponseCodeHandler handles custom response codes.
type ResponseCodeHandler func(code string, arg string)

// ExtensionHandlers allows extensions to register custom handlers.
type ExtensionHandlers struct {
	// Response handlers for untagged responses keyed by response name.
	Response map[string]ResponseHandler
	// ResponseCode handlers for response codes keyed by code name.
	ResponseCode map[string]ResponseCodeHandler
}

// NewExtensionHandlers creates a new ExtensionHandlers.
func NewExtensionHandlers() *ExtensionHandlers {
	return &ExtensionHandlers{
		Response:     make(map[string]ResponseHandler),
		ResponseCode: make(map[string]ResponseCodeHandler),
	}
}

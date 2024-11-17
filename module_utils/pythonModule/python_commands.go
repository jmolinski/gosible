package pythonModule

const (
	cmdHello         string = "hello"
	cmdExecuteModule string = "execute"
)

type Tag = uint64

type requestHeader struct {
	Cmd string
	Tag Tag
}
type responseHeader struct {
	Tag Tag
}

type helloRequest struct {
}
type helloResponse struct {
}

type executeModuleRequest struct {
	ModuleName string
	Args       interface{}
}
type executeModuleResponse struct {
	Result    map[string]interface{}
	Exception *string
}

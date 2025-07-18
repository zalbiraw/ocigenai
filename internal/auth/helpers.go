package auth

const (
	requestHeaderXContentSHA256 = "X-Content-SHA256"
	requestHeaderContentLength  = "Content-Length"
	requestHeaderAuthorization  = "Authorization"
)

func makeACopy(original []string) []string {
	tmp := make([]string, len(original))
	copy(tmp, original)
	return tmp
}

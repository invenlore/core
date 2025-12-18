publish:
	GOPROXY=proxy.golang.org go list -m github.com/invenlore/core@v$(v)

export GOPATH := $(PWD)

.PHONY: all clean install

all: $(GOPATH)/bin/phoenix-ga

$(GOPATH)/bin/phoenix-ga:
	go install -v -tags netgo -a phoenix-ga

install: all
	install -v -m0750 -t $(DESTDIR)/usr/sbin $(GOPATH)/bin/phoenix-ga

clean:
	rm -Rf $(GOPATH)/bin $(GOPATH)/pkg

fmt:
	gofmt -w $(GOPATH)/src/phoenix-ga
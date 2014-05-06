export GOPATH := $(PWD)/.build

.PHONY: all clean install

all: $(GOPATH)/bin/phoenix-ga

$(GOPATH)/bin/phoenix-ga: $(GOPATH)/fresh
	go install -v -tags netgo -a phoenix-ga

install: all
	install -v -m0750 -t $(DESTDIR)/usr/sbin $(GOPATH)/bin/phoenix-ga

$(GOPATH)/fresh:
	install -v -d $(GOPATH)/bin $(GOPATH)/pkg
	cp -at $(GOPATH)/ src

clean:
	rm -rf $(GOPATH)

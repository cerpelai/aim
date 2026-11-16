PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
TARGET ?= aim

.PHONY: all build install clean

all: install

build:
	go build -o $(TARGET) ./cmd/aim

install: build
	mkdir -p $(BINDIR)
	install -m 755 $(TARGET) $(BINDIR)/$(TARGET)

clean:
	rm -f $(TARGET)

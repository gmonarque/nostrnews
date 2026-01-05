.PHONY: all build website clean

all: build website

build:
	go build -o nostrnews .

website: website/index.html
	@python3 scripts/update_filters.py

clean:
	rm -f nostrnews published.db

run: build
	@echo "Run with: NOSTR_PRIVATE_KEY=your_hex_key ./nostrnews"


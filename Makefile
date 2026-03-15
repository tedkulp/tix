# This Makefile proxies all targets to just (https://just.systems)
# Install just: https://just.systems/man/en/chapter_4.html

%:
	just $@

.DEFAULT_GOAL := default

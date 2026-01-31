CXX ?= g++
CXXFLAGS ?= -std=c++17 -Wall -Wextra -pedantic -DASIO_STANDALONE

UNAME_S := $(shell uname -s)
SRC_DIR := node
BUILD_DIR := build
TARGET := $(BUILD_DIR)/node_server
SOURCES := $(SRC_DIR)/main.cpp

# Optional vcpkg include path (manifest default is ./vcpkg_installed).
VCPKG_INSTALLED_DIR ?= $(CURDIR)/vcpkg_installed
UNAME_M := $(shell uname -m)
VCPKG_TRIPLET ?= x64-linux
ifeq ($(UNAME_S),Darwin)
ifneq (,$(findstring arm64,$(UNAME_M)))
VCPKG_TRIPLET := arm64-osx
else
VCPKG_TRIPLET := x64-osx
endif
endif
VCPKG_INCLUDE := $(VCPKG_INSTALLED_DIR)/$(VCPKG_TRIPLET)/include
ifneq ($(wildcard $(VCPKG_INCLUDE)),)
CXXFLAGS += -I$(VCPKG_INCLUDE)
endif
LDFLAGS ?= -pthread

.PHONY: all clean

all: $(TARGET)

$(BUILD_DIR):
	@mkdir -p $@

$(TARGET): $(SOURCES) | $(BUILD_DIR)
	$(CXX) $(CXXFLAGS) -o $@ $^ $(LDFLAGS)

clean:
	@rm -rf $(BUILD_DIR)

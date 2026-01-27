//go:build darwin && hidvirtual

#include "HIDVirtualDevice/HIDVirtualDeviceBridge.h"
#include <stdint.h>
#include <stdlib.h>

// Forward declaration of Go function
extern void HIDReceiveReport(uint8_t* data, size_t length);

// Wrapper to match the bridge callback signature (const uint8_t*) and call Go
void bridgeCallbackWrapper(const uint8_t *data, size_t length) {
  HIDReceiveReport((uint8_t *)data, length);
}

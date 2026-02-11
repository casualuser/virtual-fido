
#include "_cgo_export.h"
#include "output/include/USBDriverLib/USBDriverLib.h"
#include <stdlib.h>
#include <string.h>

static usb_driver_device_t *device;

void receive_data(usb_driver_device_t *device, usb_driver_hid_frame_t *frame) {
  receiveDataCallback(frame->data, frame->length);
}

void send_data(void *data, int length) {
  usb_driver_hid_frame_t frame;
  memcpy(frame.data, data, length);
  frame.length = length;
  usb_driver_send_frame(device, &frame);
}

void start_device(void) {
  fprintf(stderr, "[C] Starting device initialization...\n");
  device = usb_driver_init_device(receive_data);
  if (!device) {
    fprintf(stderr, "[C] ERROR: usb_driver_init_device returned NULL\n");
    return;
  }
  fprintf(stderr, "[C] Device initialized. Starting driver...\n");
  usb_driver_start(device);
  fprintf(stderr, "[C] usb_driver_start called.\n");
}

void stop_device(void) {
  if (device) {
    usb_driver_stop(device);
    free(device);
    device = NULL;
  }
}
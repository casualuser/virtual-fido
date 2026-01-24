
#include "_cgo_export.h"
#include "output/include/USBDriverLib/USBDriverLib.h"
#include <stdio.h> // Added for fprintf
#include <string.h>
#include <unistd.h>

static usb_driver_device_t *device;

void receive_data(usb_driver_device_t *device, usb_driver_hid_frame_t *frame) {
  receiveDataCallback(frame->data, frame->length);
}

void send_data(void *data, int length) {
  if (length > 64) {
    length = 64;
  }

  // Add small delay to prevent overflowing the OS HID queue
  usleep(5000);

  usb_driver_hid_frame_t frame;
  memset(&frame, 0, sizeof(frame));
  memcpy(frame.data, data, length);
  frame.length = length;
  usb_driver_send_frame(device, &frame);
}

void start_device(void) {
  fprintf(stderr, "DEBUG: C start_device called\n");
  fflush(stderr);
  device = usb_driver_init_device(receive_data);
  usb_driver_start(device);
}
#include "_cgo_export.h"
#include "output/include/USBDriverLib/USBDriverLib.h"
#include <unistd.h>

static usb_driver_device_t *device;

void receive_data(usb_driver_device_t *device, usb_driver_hid_frame_t *frame) {
  receiveDataCallback(frame->data, frame->length);
}

void send_data(void *data, int length) {
  if (length > 64) {
    printf("USBDriverLib: send_data length %d > 64, truncating\n", length);
    length = 64;
  }

  usleep(5000); // Prevent overflowing the OS HID queue, increased to 5ms

  usb_driver_hid_frame_t frame;
  memset(&frame, 0, sizeof(frame));
  memcpy(frame.data, data, length);
  frame.length = length;
  usb_driver_send_frame(device, &frame);
}

void start_device(void) {
  device = usb_driver_init_device(receive_data);
  usb_driver_start(device);
}
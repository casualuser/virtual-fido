#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <stdio.h>
#include <unistd.h>

// Matches USBDriver personality name
#define SERVICE_NAME "USBDriver"

int main() {
  kern_return_t ret;
  io_service_t service = IO_OBJECT_NULL;
  io_connect_t connection = IO_OBJECT_NULL;

  printf("--- Stage 1: Service Discovery ---\n");
  // Initial sleep to allow DEXT to settle
  printf("Sleeping 2 seconds for DEXT stability...\n");
  sleep(2);

  service = IOServiceGetMatchingService(kIOMainPortDefault,
                                        IOServiceNameMatching(SERVICE_NAME));
  if (!service) {
    printf("FAILED: Could not find service with name '%s'\n", SERVICE_NAME);
    return 1;
  }
  printf("SUCCESS: Found service '%s'\n", SERVICE_NAME);

  printf("\n--- Stage 2: Connection Retries ---\n");
  // Try to open connection with 20 retries and 500ms sleep
  for (int i = 1; i <= 20; i++) {
    printf("Attempt %d of 20 (ConnectType 0)...\n", i);
    // kIOHIDServerConnectType = 0
    ret = IOServiceOpen(service, mach_task_self_, 0, &connection);

    if (ret == kIOReturnSuccess) {
      printf("SUCCESS: Connection opened!\n");
      IOServiceClose(connection);
      break;
    } else {
      printf("FAILED: 0x%08x (Sys:0x%x, Sub:0x%x, Code:0x%x)\n", ret,
             err_get_system(ret), err_get_sub(ret), err_get_code(ret));

      if (ret == 0xe00002e2) {
        printf("  -> kIOReturnNotPermitted (Permission error)\n");
      } else if (ret == 0xe00002c1) {
        printf("  -> kIOReturnUnsupported (Type/method mismatch?)\n");
      } else if (ret == 0xe00002bc) {
        printf("  -> kIOReturnNoDevice\n");
      }
    }
    usleep(500000); // 500ms
  }

  IOObjectRelease(service);
  return 0;
}

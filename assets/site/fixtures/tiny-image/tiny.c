#include <unistd.h>
#include <stdio.h>

int main() {
    while (1) {
        printf("Sleepig for 5 seconds...\n");
        sleep(5);
    }
    return 0;
}

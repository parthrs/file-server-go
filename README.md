#### Overview
A small learning exercise by writing a basic file server in Go

#### Build
`docker build -t file-server-go .`

#### Run
`docker run --memory="64m" --memory-swap="100m" -p 37899:37899 file-server-go`

##### Observations:
- Running the `free -h` command from within the container will show incorrect results (see [this](https://stackoverflow.com/a/72185762/768020) answer)
    
    For instance, with the memory flag set to `64M` and the swap flag set to `100M` (effective swap is `100 - 64` = `36M`);

    `free -h` (from inside container):
    ```
                  total        used        free      shared  buff/cache   available
    Mem:           2.9G        1.4G      259.6M       29.4M        1.2G        1.1G
    Swap:        512.0M      200.0M      312.0M
    ```

    `docker inspect <container-id>` (from the host, i.e. outside the container)
    ```
    CONTAINER ID   NAME               CPU %     MEM USAGE / LIMIT   MEM %     NET I/O       BLOCK I/O        PIDS
    8300bf4be0ba   quizzical_panini   0.00%     1.465MiB / 64MiB    2.29%     1.09kB / 0B   266kB / 12.3kB   7
    ```

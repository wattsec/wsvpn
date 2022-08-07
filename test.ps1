#!/usr/bin/env pwsh

docker build -t wsvpn-test ./testcontainer
docker run -e GITHUB_ACTIONS --cap-add=NET_ADMIN --device /dev/net/tun:/dev/net/tun -v "/dev/net/tun:/dev/net/run" -v "${pwd}:/mnt:ro" -it wsvpn-test $args
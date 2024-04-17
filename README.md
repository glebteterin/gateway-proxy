# Gateway Proxy

Package `gateway` provides a simple reverse proxy server that can switch between two backends based on the response of the first one.

## Overview

The `gateway` package offers functionality to create a reverse proxy server capable of routing requests to two different backend servers (`A` and `B`).<br />
It dynamically switches between the backends based on the response received from the initial request to `A`.<br />
If `A` responds with a 404 error and a special header indicating a fallback route, subsequent requests are routed to `B`.

**Note**: Routes are cached for performance. To reset the cache, you must restart the server.

## Features

* Dynamic Routing: The server dynamically switches between two backend servers based on the response from the initial request.
* Fallback Mechanism: If the primary backend (A) returns a 404 error with a specific header indicating a fallback route, subsequent requests are routed to the secondary backend (B).
* Route Caching: Routes are cached for performance. To reset the cache, you must restart the server.

## License

This project is licensed under the MIT License.

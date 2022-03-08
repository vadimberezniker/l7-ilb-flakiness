This repository demonstrates issues with long-running gRPC streams going through GCP L7 ILB. 

My observations point to a relatively frequent churn of Envoys causing long-running gRPC streams to be broken.

The repro below seems to show that there's an Envoy host churning on average every 9 minutes causing any long-running connections going through that Envoy to be broken.

This repro contains a gRPC client and gRPC server using a mix of short and long-running RPCs. 

To run this repro, you will need two VMs for the server and the client and a L7 ILB. I used c2-standard-4 VMs for my testing. The L7 ILB should be configured to healthcheck on port 8080 and to send traffic to the backend on port 1986. The load balancer should be configured with a long timeout to avoid terminating RPCs prematurely. 

The binaries can be built using Bazel (//server and //client targets).

1. Run the server binary on the server VM.

Every minute, the server will dump list of RPC peers seen in the last minute. After a long-running stream is broken, you can observe that a peer will drop off from this list. 

2. Use the client binary on the client VM to generate 10k qps of fast RPCs:

`./client --server <ilb address>:443 --mode ping --concurrency 100 --delay 10ms`

Let this run for a minute or so since it appears that the traffic will cause the number of Envoys to be scaled up. Leave this instance of the binary running on the server to continue generating traffic.

3. Use the client binary on the client VM to establish long-running streaming RPCs:

`./client --server <ilb address>:443 --mode register --concurrency 100`

This will establish 100 long-running streaming RPCs. The client will log whenever one of these long running RPCs is broken.
It will also keep track of the amount of time before any stream is broken. After leaving this running for 5 hours, I have the following output:

`2022/03/12 00:14:58 Average time between disconnects: 9.505050m`



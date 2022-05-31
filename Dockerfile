# Build the go application into a binary
FROM golang:alpine as builder
WORKDIR /app
ADD . ./
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -a -installsuffix cgo -o aws-eks-asg-rolling-update-handler .
RUN apk --update add ca-certificates

# Run the binary on an empty container
FROM scratch
COPY --from=builder /app/aws-eks-asg-rolling-update-handler .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/aws-eks-asg-rolling-update-handler"]
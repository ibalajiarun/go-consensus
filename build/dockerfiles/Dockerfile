FROM balajia/sgx-go:latest as builder

RUN go get -u github.com/gogo/protobuf/protoc-gen-gogofaster && \
    go get -u github.com/golang/protobuf/protoc-gen-go

ENV libbls https://github.com/ibalajiarun/sgxwallet/releases/download/v0.4/libbls.tar.gz
RUN wget -O /root/libbls.tar.gz ${libbls} && \
    mkdir -p /opt/libbls && \
    tar -C /opt/libbls -xzf /root/libbls.tar.gz && \
    rm /root/libbls.tar.gz

ENV libpolycrypto https://github.com/ibalajiarun/libpolycrypto/releases/download/v0.2/libpolycrypto.tar.gz
RUN wget -O /root/libpolycrypto.tar.gz ${libpolycrypto} && \
    mkdir -p /opt/libpolycrypto && \
    tar -C /opt/libpolycrypto -xzf /root/libpolycrypto.tar.gz && \
    rm /root/libpolycrypto.tar.gz

WORKDIR /app
COPY . .

ENV SGX_SDK=/opt/sgxsdk
ENV PATH=$PATH:$SGX_SDK/bin:$SGX_SDK/bin/x64 
ENV PKG_CONFIG_PATH=$PKG_CONFIG_PATH:$SGX_SDK/pkgconfig 
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$SGX_SDK/sdk_libs

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64 
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/app/utils/sgx/enclave_trinc:/app/utils/sgx/enclave_trudep
ENV SGX_MODE=HW
RUN make build

###############################################################################

FROM balajia/sgx-runtime:latest
LABEL org.opencontainers.image.source https://github.com/ibalajiarun/go-consensus

RUN apt-get update && \
    apt-get install -y libgomp1 && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /var/cache/apt/archives/*

COPY --from=builder /app/enclaves/trinx/enclave_trinx/libtrinx.so /lib/libtrinx.so
COPY --from=builder /app/enclaves/trinx/enclave_trinx/trinxenclave.signed.so /lib/trinxenclave.signed.so
COPY --from=builder /app/enclaves/threshsign/enclave_threshsign/libthreshsign.so /lib/libthreshsign.so
COPY --from=builder /app/enclaves/threshsign/enclave_threshsign/threshsignenclave.signed.so /lib/threshsignenclave.signed.so
COPY --from=builder /app/enclaves/usig/enclave_usig/libusig.so /lib/libusig.so
COPY --from=builder /app/enclaves/usig/enclave_usig/usig_enclave.signed.so /lib/usig_enclave.signed.so
COPY --from=builder /app/enclaves/threshsign2/enclave_threshsign/libthreshsign2.so /lib/libthreshsign2.so
COPY --from=builder /app/enclaves/threshsign2/enclave_threshsign/threshsign2enclave.signed.so /lib/threshsign2enclave.signed.so

COPY --from=builder /app/bin/peer /bin/peer
COPY --from=builder /app/bin/client /bin/client
EXPOSE 7000
ENTRYPOINT ["/bin/peer"]
CERTS = etcd1.crt etcd2.crt etcd3.crt proxy1.crt

.PHONY: all
all: $(CERTS)

.PHONY: ca
ca: ca.crt

%.key:
	openssl genrsa -out $@ 2048

ca.csr: ca.key ca.config
	openssl req -new -key $< -out $@ -config ca.config

ca.crt: ca.csr ca.key
	cat ca.csr | openssl x509 -req -days 1095 -sha256 -keyform pem -signkey ca.key -set_serial 0x2EC9165BAC1CFD93B558790F8EB291D99E359072 -out ca.crt

%.csr: %.key crt.config
	openssl req -new -key $< -out $@ -config crt.config

%.crt: %.csr %.key crt.config ca.crt ca.key
	openssl x509 -req -days 1095 -CA ca.crt -CAkey ca.key -CAcreateserial -in $*.csr -out $*.crt -extfile crt.config -extensions 'v3_req'


.PHONY: rotate-ca
rotate-ca:
	rm ca.crt
	$(MAKE) ca.crt

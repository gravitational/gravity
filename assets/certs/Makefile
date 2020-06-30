all:
	for f in etcd1 etcd2 etcd3 proxy1; do \
		openssl req -new -key $$f-key.pem -out $$f.csr -config config; \
		openssl x509 -req -days 1000 -CA ca.pem -CAkey ca-key.pem -in $$f.csr -out $$f.pem -extfile config -extensions 'v3_req'; \
	done

where-am-i = $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))
TEST_ETCD_CERTS := $(realpath $(dir $(call where-am-i))../assets/certs)
TEST_ETCD_CONFIG := '{"nodes": ["https://localhost:4001"], "key":"/gravity/test", "tls_key_file": "$(TEST_ETCD_CERTS)/proxy1-key.pem", "tls_cert_file": "$(TEST_ETCD_CERTS)/proxy1.pem", "tls_ca_file": "$(TEST_ETCD_CERTS)/ca.pem"}'
TEST_ETCD_IMAGE := quay.io/coreos/etcd:v$(ETCD_VER)
TEST_ETCD_INSTANCE := testetcd0

ifeq ($(TEST_ETCD_STATEFUL),yes)
TEST_ETCD_FLAGS := --data-dir=/var/lib/etcd/data --wal-dir=/var/lib/etcd/wal
TEST_ETCD_MOUNTS := -v /var/lib/gravity/etcd:/var/lib/etcd
else
endif

.PHONY: base-etcd
base-etcd:
	if docker ps | grep $(TEST_ETCD_INSTANCE) -q; then \
	  echo "ETCD is already running"; \
	else \
	  echo "starting test ETCD instance"; \
	  etcd_instance=$(shell docker ps -a | grep $(TEST_ETCD_INSTANCE) | awk '{print $$1}'); \
	  if [ "$$etcd_instance" != "" ]; then \
	    docker rm -v $$etcd_instance; \
	  fi; \
	  docker run --net=host $(TEST_ETCD_MOUNTS) --name=$(TEST_ETCD_INSTANCE) -d -v $(TEST_ETCD_CERTS):/certs $(TEST_ETCD_IMAGE)  -name etcd0 -advertise-client-urls https://localhost:2379,https://localhost:4001  -listen-client-urls https://0.0.0.0:2379,https://0.0.0.0:4001  -initial-advertise-peer-urls https://localhost:2380  -listen-peer-urls https://0.0.0.0:2380  -initial-cluster-token etcd-cluster-1  -initial-cluster etcd0=https://localhost:2380  -initial-cluster-state new --cert-file=/certs/etcd1.pem --key-file=/certs/etcd1-key.pem --peer-cert-file=/certs/etcd1.pem --peer-key-file=/certs/etcd1-key.pem --peer-client-cert-auth --peer-trusted-ca-file=/certs/ca.pem -client-cert-auth --trusted-ca-file=/certs/ca.pem $(TEST_ETCD_FLAGS) ; \
	fi;

.PHONY: etcd
etcd:
	$(MAKE) base-etcd TEST_ETCD_STATEFUL=yes

.PHONY: test-etcd
test-etcd:
	$(MAKE) base-etcd

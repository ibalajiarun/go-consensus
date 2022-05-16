SGX_MODE ?= HW
SGX_DEBUG ?= 1
TARGET ?= lib

export SGX_MODE

ENCLAVES := usig/enclave_usig trinx/enclave_trinx threshsign/enclave_threshsign threshsign2/enclave_threshsign

.PHONY: enclaves $(ENCLAVES)
enclaves: $(ENCLAVES)

$(ENCLAVES):
	@$(MAKE) -C $@ $(TARGET)
/*
Copyright 2018-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package schema

import (
	"log"
	"strings"

	"github.com/santhosh-tekuri/jsonschema"
)

var schema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft6
	compiler.ExtractAnnotations = true
	if err := compiler.AddResource("schema.json", strings.NewReader(manifestSchema)); err != nil {
		log.Fatalf("Failed to add schema resource: %v.", err)
	}

	var err error
	schema, err = compiler.Compile("schema.json")
	if err != nil {
		log.Fatalf("Failed to parse schema: %v.", err)
	}
}

const manifestSchema = `
{
  "$schema": "http://json-schema.org/draft-06/schema#",
  "description": "Gravity Application Manifest Schema v2",
  "$ref": "#/definitions/Manifest",
  "definitions": {
    "Manifest": {
      "type": "object",
      "required": ["apiVersion", "kind", "metadata"],
      "additionalProperties": false,
      "properties": {
        "apiVersion": {"type": "string"},
        "kind": {"type": "string"},
        "metadata": {
          "type": "object",
          "required": ["name", "resourceVersion"],
          "additionalProperties": false,
          "properties": {
            "name": {"type": "string"},
            "resourceVersion": {"type": "string"},
            "namespace": {"type": "string", "default": "default"},
            "repository": {"type": "string", "default": "gravitational.io"},
            "description": {"type": "string"},
            "author": {"type": "string"},
            "createdTimestamp": {"type": "string"},
            "hidden": {"type": "boolean"},
            "labels": {"type": "object"}
          }
        },
        "baseImage": {"type": "string"},
        "logo": {"type": "string"},
        "releaseNotes": {"type": "string"},
        "endpoints": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name"],
            "additionalProperties": false,
            "properties": {
              "name": {"type": "string"},
              "description": {"type": "string"},
              "selector": {"type": "object"},
              "serviceName": {"type": "string"},
              "namespace": {"type": "string"},
              "protocol": {"type": "string"},
              "port": {"type": "number"},
              "hidden": {"type": "boolean"}
            }
          }
        },
        "dependencies": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "packages": {
              "type": "array",
              "items": {"type": "string"}
            },
            "apps": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "installer": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "eula": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "source": {"type": "string"}
              }
            },
            "setupEndpoints": {
              "type": "array",
              "items": {"type": "string"}
            },
            "flavors": {
              "type": "object",
              "required": ["items"],
              "additionalProperties": false,
              "properties": {
                "prompt": {"type": "string"},
                "description": {"type": "string"},
                "default": {"type": "string"},
                "items": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "required": ["name", "nodes"],
                    "additionalProperties": false,
                    "properties": {
                      "name": {"type": "string"},
                      "description": {"type": "string"},
                      "nodes": {
                        "type": "array",
                        "items": {
                          "type": "object",
                          "required": ["profile", "count"],
                          "additionalProperties": false,
                          "properties": {
                            "profile": {"type": "string"},
                            "count": {"type": "number"}
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        },
        "nodeProfiles": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name"],
            "additionalProperties": false,
            "properties": {
              "name": {"type": "string"},
              "description": {"type": "string"},
              "systemOptions": {"$ref": "#/definitions/systemOptions"},
              "requirements": {
                "type": "object",
                "additionalProperties": false,
                "properties": {
                  "cpu": {
                    "type": "object",
                    "additionalProperties": false,
                    "properties": {
                      "min": {"type": "number"},
                      "max": {"type": "number"}
                    }
                  },
                  "ram": {
                    "type": "object",
                    "additionalProperties": false,
                    "properties": {
                      "min": {"type": "string"},
                      "max": {"type": "string"}
                    }
                  },
                  "os": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "required": ["name"],
                      "additionalProperties": false,
                      "properties": {
                        "name": {"type": "string"},
                        "versions": {
                          "type": "array",
                          "items": {"type": "string"}
                        }
                      }
                    }
                  },
                  "network": {
                    "type": "object",
                    "additionalProperties": false,
                    "properties": {
                      "minTransferRate": {"type": "string"},
                      "ports": {
                        "type": "array",
                        "items": {
                          "type": "object",
                          "required": ["ranges"],
                          "additionalProperties": false,
                          "properties": {
                            "protocol": {"type": "string", "default": "tcp"},
                            "ranges": {
                              "type": "array",
                              "items": {"type": "string"}
                            }
                          }
                        }
                      }
                    }
                  },
                  "volumes": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "required": ["path"],
                      "additionalProperties": false,
                      "properties": {
                        "name": {"type": "string"},
                        "path": {"type": "string"},
                        "targetPath": {"type": "string"},
                        "capacity": {"type": "string"},
                        "filesystems": {
                          "type": "array",
                          "items": {"type": "string"}
                        },
                        "createIfMissing": {"type": "boolean", "default": true},
                        "skipIfMissing": {"type": "boolean", "default": false},
                        "minTransferRate": {"type": "string"},
                        "hidden": {"type": "boolean"},
                        "recursive": {"type": "boolean"},
                        "mode": {"type": "string"},
                        "uid": {"type": "number"},
                        "gid": {"type": "number"},
                        "label": {"type": "string"}
                      }
                    }
                  },
                  "devices": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "required": ["path"],
                      "additionalProperties": false,
                      "properties": {
                        "path": {"type": "string"},
                        "permissions": {"type": "string", "default": "rw"},
                        "fileMode": {"type": "string", "default": "0666"},
                        "uid": {"type": "number", "default": 0},
                        "gid": {"type": "number", "default": 0}
                      }
                    }
                  },
                  "customChecks": {
                    "type": "array",
                    "items": {
                      "type": "object",
                      "properties": {
                        "description": {"type": "string"},
                        "script": {"type": "string"}
                      }
                    }
                  }
                }
              },
              "labels": {"type": "object"},
              "taints": {
                "type": "array",
                "items": {
                  "type": "object",
                  "required": ["key", "effect"],
                  "additionalProperties": false,
                  "properties": {
                    "key": {"type": "string"},
                    "value": {"type": "string"},
                    "effect": {"type": "string"}
                   }
                }
              },
              "providers": {
                "type": "object",
                "additionalProperties": false,
                "properties": {
                  "aws": {
                    "type": "object",
                    "additionalProperties": false,
                    "properties": {
                      "instanceTypes": {
                        "type": "array",
                        "items": {"type": "string"}
                      }
                    }
                  }
                }
              },
              "expandPolicy": {"type": "string"},
              "serviceRole": {"type": "string"}
            }
          }
        },
        "providers": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "default": {"type": "string"},
            "aws": {"$ref": "#/definitions/providerAWS"},
            "azure": {"$ref": "#/definitions/providerAzure"},
            "generic": {"$ref": "#/definitions/providerGeneric"}
          }
        },
        "storage": {
          "type": "object",
          "properties": {
            "openebs": {
              "type": "object",
              "properties": {
                "enabled": {"type": "boolean"}
              }
            }
          }
        },
        "license": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "enabled": {"type": "boolean"},
            "type": {"type": "string", "default": "certificate"}
          }
        },
        "hooks": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "clusterProvision": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "clusterProvision"},
                "job": {"type": "string"}
              }
            },
            "clusterDeprovision": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "clusterDeprovision"},
                "job": {"type": "string"}
              }
            },
            "nodesProvision": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "nodesProvision"},
                "job": {"type": "string"}
              }
            },
            "nodesDeprovision": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "nodesDeprovision"},
                "job": {"type": "string"}
              }
            },
            "install": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "install"},
                "job": {"type": "string"}
              }
            },
            "postInstall": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "postInstall"},
                "job": {"type": "string"}
              }
            },
            "uninstall": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "uninstall"},
                "job": {"type": "string"}
              }
            },
            "preUninstall": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "preUninstall"},
                "job": {"type": "string"}
              }
            },
            "preNodeAdd": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "preNodeAdd"},
                "job": {"type": "string"}
              }
            },
            "postNodeAdd": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "postNodeAdd"},
                "job": {"type": "string"}
              }
            },
            "preNodeRemove": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "preNodeRemove"},
                "job": {"type": "string"}
              }
            },
            "postNodeRemove": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "postNodeRemove"},
                "job": {"type": "string"}
              }
            },
            "preUpdate": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "preUpdate"},
                "job": {"type": "string"}
              }
            },
            "update": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "update"},
                "job": {"type": "string"}
              }
            },
            "postUpdate": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "postUpdate"},
                "job": {"type": "string"}
              }
            },
            "rollback": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "rollback"},
                "job": {"type": "string"}
              }
            },
            "postRollback": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "postRollback"},
                "job": {"type": "string"}
              }
            },
            "status": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "status"},
                "job": {"type": "string"}
              }
            },
            "info": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "info"},
                "job": {"type": "string"}
              }
            },
            "licenseUpdated": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "licenseUpdated"},
                "job": {"type": "string"}
              }
            },
            "start": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "start"},
                "job": {"type": "string"}
              }
            },
            "stop": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "stop"},
                "job": {"type": "string"}
              }
            },
            "dump": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "dump"},
                "job": {"type": "string"}
              }
            },
            "backup": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "backup"},
                "job": {"type": "string"}
              }
            },
            "restore": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "restore"},
                "job": {"type": "string"}
              }
            },
            "networkInstall": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "networkInstall"},
                "job": {"type": "string"}
              }
            },
            "networkUpdate": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "networkUpdate"},
                "job": {"type": "string"}
              }
            },
            "networkRollback": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "type": {"type": "string", "default": "networkRollback"},
                "job": {"type": "string"}
              }
            }
          }
        },
        "systemOptions": {"$ref": "#/definitions/systemOptions"},
        "extensions": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "encryption": {
              "type": "object",
              "required": ["encryptionKey", "caCert"],
              "additionalProperties": false,
              "properties": {
                "encryptionKey": {"type": "string"},
                "caCert": {"type": "string"}
              }
            },
            "logs": {"$ref": "#/definitions/onOff"},
            "monitoring": {"$ref": "#/definitions/onOff"},
            "catalog": {"$ref": "#/definitions/onOff"},
            "kubernetes": {"$ref": "#/definitions/onOff"},
            "configuration": {"$ref": "#/definitions/onOff"}
          }
        },
        "webConfig": {"type": "string"}
      }
    },
    "providerAWS": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "network": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "type": {"type": "string"}
          }
        },
        "terraform": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "script": {"type": "string"},
            "instanceScript": {"type": "string"}
          }
        },
        "regions": {
          "type": "array",
          "items": {"type": "string"}
        },
        "iamPolicy": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "version": {"type": "string", "default": "2012-10-17"},
            "actions": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "disabled": {"type": "boolean"}
      }
    },
    "providerAzure": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "disabled": {"type": "boolean"}
      }
    },
    "providerGeneric": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "network": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "type": {"type": "string"}
          }
        },
        "disabled": {"type": "boolean"}
      }
    },
    "systemOptions": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "baseImage": {"type": "string"},
        "allowPrivileged": {"type": "boolean"},
        "args": {
          "type": "array",
          "items": {"type": "string"}
        },
        "runtime": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "name": {"type": "string", "default": "kubernetes"},
            "version": {"type": "string", "default": "0.0.0+latest"},
            "repository": {"type": "string", "default": "gravitational.io"}
          }
        },
        "docker": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "storageDriver": {"type": "string"},
            "capacity": {"type": "string"},
            "args": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "kubelet": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "hairpinMode": {"enum": ["promiscuous-bridge", "hairpin-veth"], "default": ""},
            "args": {
              "type": "array",
              "items": {"type": "string"}
            }
          }
        },
        "etcd": {"$ref": "#/definitions/externalService"},
        "dependencies": {
          "type": "object",
          "properties": {
            "runtimePackage": {"type": "string"}
          }
        }
      }
    },
    "externalService": {
      "type": "object",
      "description": "Additional command line for an external service",
      "properties": {
        "args": {
          "type": "array",
          "items": {"type": "string"}
        }
      }
    },
    "onOff": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "disabled": {"type": "boolean"}
      }
    }
  }
}
`

// This file was generated with:
// tfk8s -f db-operator-single-file-deploy.yaml -p "kubernetes-alpha" --strip -o main.tf
// Afterwards manually add: "image" = var.image

resource "kubernetes_manifest" "namespace_db_operator_system" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "v1"
    "kind" = "Namespace"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "manager"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "system"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "namespace"
        "app.kubernetes.io/part-of" = "db-operator"
        "control-plane" = "controller-manager"
      }
      "name" = "db-operator-system"
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_backupcronjobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "backupcronjobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "BackupCronJob"
        "listKind" = "BackupCronJobList"
        "plural" = "backupcronjobs"
        "singular" = "backupcronjob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "BackupCronJob is the Schema for the backupcronjobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "backup_target" = {
                      "type" = "string"
                    }
                    "fixed_file_name" = {
                      "type" = "string"
                    }
                    "interval" = {
                      "type" = "string"
                    }
                    "suspend" = {
                      "type" = "boolean"
                    }
                  }
                  "required" = [
                    "backup_target",
                    "interval",
                    "suspend",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "BackupCronJobStatus defines the observed state of BackupCronJob"
                  "properties" = {
                    "cronjob_name" = {
                      "type" = "string"
                    }
                    "exists" = {
                      "type" = "boolean"
                    }
                  }
                  "required" = [
                    "cronjob_name",
                    "exists",
                  ]
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_backupjobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "backupjobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "BackupJob"
        "listKind" = "BackupJobList"
        "plural" = "backupjobs"
        "singular" = "backupjob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "BackupJob is the Schema for the backupjobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "backup_target" = {
                      "type" = "string"
                    }
                    "fixed_file_name" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "backup_target",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "BackupJobStatus defines the observed state of BackupJob"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_backuptargets_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "backuptargets.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "BackupTarget"
        "listKind" = "BackupTargetList"
        "plural" = "backuptargets"
        "singular" = "backuptarget"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "BackupTarget is the Schema for the backuptargets API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "db_name" = {
                      "type" = "string"
                    }
                    "storage_location" = {
                      "type" = "string"
                    }
                    "storage_type" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "db_name",
                    "storage_location",
                    "storage_type",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "BackupTargetStatus defines the observed state of BackupTarget"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_dbcopycronjobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "dbcopycronjobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "DbCopyCronJob"
        "listKind" = "DbCopyCronJobList"
        "plural" = "dbcopycronjobs"
        "singular" = "dbcopycronjob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "DbCopyCronJob is the Schema for the dbcopycronjobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "description" = "DbCopyCronJobSpec defines the desired state of DbCopyCronJob"
                  "properties" = {
                    "from_db_name" = {
                      "type" = "string"
                    }
                    "interval" = {
                      "type" = "string"
                    }
                    "suspend" = {
                      "type" = "boolean"
                    }
                    "to_db_name" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "from_db_name",
                    "interval",
                    "suspend",
                    "to_db_name",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "DbCopyCronJobStatus defines the observed state of DbCopyCronJob"
                  "properties" = {
                    "cronjob_name" = {
                      "type" = "string"
                    }
                    "exists" = {
                      "type" = "boolean"
                    }
                  }
                  "required" = [
                    "cronjob_name",
                    "exists",
                  ]
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_dbcopyjobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "dbcopyjobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "DbCopyJob"
        "listKind" = "DbCopyJobList"
        "plural" = "dbcopyjobs"
        "singular" = "dbcopyjob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "DbCopyJob is the Schema for the dbcopyjobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "from_db_name" = {
                      "type" = "string"
                    }
                    "to_db_name" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "from_db_name",
                    "to_db_name",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "DbCopyJobStatus defines the observed state of DbCopyJob"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_dbs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "dbs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "Db"
        "listKind" = "DbList"
        "plural" = "dbs"
        "singular" = "db"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "Db is the Schema for the dbs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "description" = "DbSpec defines the desired state of Db"
                  "properties" = {
                    "db_name" = {
                      "type" = "string"
                    }
                    "drop_on_deletion" = {
                      "type" = "boolean"
                    }
                    "server" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "db_name",
                    "drop_on_deletion",
                    "server",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "DbStatus defines the observed state of Db"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_dbservers_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "dbservers.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "DbServer"
        "listKind" = "DbServerList"
        "plural" = "dbservers"
        "singular" = "dbserver"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "DbServer is the Schema for the dbservers API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "address" = {
                      "description" = "Server address"
                      "type" = "string"
                    }
                    "port" = {
                      "description" = "Server port"
                      "minimum" = 1
                      "type" = "integer"
                    }
                    "secret_key" = {
                      "type" = "string"
                    }
                    "secret_name" = {
                      "minLength" = 1
                      "type" = "string"
                    }
                    "server_type" = {
                      "type" = "string"
                    }
                    "user_name" = {
                      "minLength" = 1
                      "type" = "string"
                    }
                    "version" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "address",
                    "port",
                    "secret_name",
                    "server_type",
                    "user_name",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "DbServerStatus defines the observed state of DbServer"
                  "properties" = {
                    "connection_available" = {
                      "type" = "boolean"
                    }
                    "databases" = {
                      "items" = {
                        "type" = "string"
                      }
                      "type" = "array"
                    }
                    "message" = {
                      "type" = "string"
                    }
                    "users" = {
                      "items" = {
                        "type" = "string"
                      }
                      "type" = "array"
                    }
                  }
                  "required" = [
                    "connection_available",
                    "databases",
                    "message",
                    "users",
                  ]
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_restorecronjobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "restorecronjobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "RestoreCronJob"
        "listKind" = "RestoreCronJobList"
        "plural" = "restorecronjobs"
        "singular" = "restorecronjob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "RestoreCronJob is the Schema for the restorecronjobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "fixed_file_name" = {
                      "type" = "string"
                    }
                    "interval" = {
                      "type" = "string"
                    }
                    "restore_target" = {
                      "type" = "string"
                    }
                    "suspend" = {
                      "type" = "boolean"
                    }
                  }
                  "required" = [
                    "interval",
                    "restore_target",
                    "suspend",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "RestoreCronJobStatus defines the observed state of RestoreCronJob"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_restorejobs_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "restorejobs.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "RestoreJob"
        "listKind" = "RestoreJobList"
        "plural" = "restorejobs"
        "singular" = "restorejob"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "RestoreJob is the Schema for the restorejobs API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "fixed_file_name" = {
                      "type" = "string"
                    }
                    "restore_target" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "restore_target",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "RestoreJobStatus defines the observed state of RestoreJob"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_restoretargets_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "restoretargets.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "RestoreTarget"
        "listKind" = "RestoreTargetList"
        "plural" = "restoretargets"
        "singular" = "restoretarget"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "RestoreTarget is the Schema for the restoretargets API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "db_name" = {
                      "type" = "string"
                    }
                    "storage_location" = {
                      "type" = "string"
                    }
                    "storage_type" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "db_name",
                    "storage_location",
                    "storage_type",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "RestoreTargetStatus defines the observed state of RestoreTarget"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_s3storages_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "s3storages.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "S3Storage"
        "listKind" = "S3StorageList"
        "plural" = "s3storages"
        "singular" = "s3storage"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "S3Storage is the Schema for the s3storages API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "properties" = {
                    "access_key_id" = {
                      "type" = "string"
                    }
                    "bucket_name" = {
                      "type" = "string"
                    }
                    "endpoint" = {
                      "type" = "string"
                    }
                    "prefix" = {
                      "type" = "string"
                    }
                    "region" = {
                      "type" = "string"
                    }
                    "secret_access_key_k8s_secret" = {
                      "type" = "string"
                    }
                    "secret_access_key_k8s_secret_key" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "access_key_id",
                    "bucket_name",
                    "region",
                    "secret_access_key_k8s_secret",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "S3StorageStatus defines the observed state of S3Storage"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "customresourcedefinition_users_db_operator_kubemaster_com" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apiextensions.k8s.io/v1"
    "kind" = "CustomResourceDefinition"
    "metadata" = {
      "annotations" = {
        "controller-gen.kubebuilder.io/version" = "v0.10.0"
      }
      "name" = "users.db-operator.kubemaster.com"
    }
    "spec" = {
      "group" = "db-operator.kubemaster.com"
      "names" = {
        "kind" = "User"
        "listKind" = "UserList"
        "plural" = "users"
        "singular" = "user"
      }
      "scope" = "Namespaced"
      "versions" = [
        {
          "name" = "v1alpha1"
          "schema" = {
            "openAPIV3Schema" = {
              "description" = "User is the Schema for the users API"
              "properties" = {
                "apiVersion" = {
                  "description" = "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources"
                  "type" = "string"
                }
                "kind" = {
                  "description" = "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
                  "type" = "string"
                }
                "metadata" = {
                  "type" = "object"
                }
                "spec" = {
                  "description" = "UserSpec defines the desired state of User"
                  "properties" = {
                    "db_privs" = {
                      "items" = {
                        "properties" = {
                          "db_name" = {
                            "type" = "string"
                          }
                          "privs" = {
                            "type" = "string"
                          }
                        }
                        "required" = [
                          "db_name",
                          "privs",
                        ]
                        "type" = "object"
                      }
                      "type" = "array"
                    }
                    "db_server_name" = {
                      "type" = "string"
                    }
                    "secret_key" = {
                      "type" = "string"
                    }
                    "secret_name" = {
                      "type" = "string"
                    }
                    "server_privs" = {
                      "type" = "string"
                    }
                    "user_name" = {
                      "type" = "string"
                    }
                  }
                  "required" = [
                    "db_privs",
                    "db_server_name",
                    "secret_name",
                    "server_privs",
                    "user_name",
                  ]
                  "type" = "object"
                }
                "status" = {
                  "description" = "UserStatus defines the observed state of User"
                  "type" = "object"
                }
              }
              "type" = "object"
            }
          }
          "served" = true
          "storage" = true
          "subresources" = {
            "status" = {}
          }
        },
      ]
    }
  }
}

resource "kubernetes_manifest" "serviceaccount_db_operator_system_db_operator_controller_manager" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "v1"
    "kind" = "ServiceAccount"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "rbac"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "controller-manager"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "serviceaccount"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-controller-manager"
      "namespace" = "db-operator-system"
    }
  }
}

resource "kubernetes_manifest" "role_db_operator_system_db_operator_leader_election_role" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "Role"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "rbac"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "leader-election-role"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "role"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-leader-election-role"
      "namespace" = "db-operator-system"
    }
    "rules" = [
      {
        "apiGroups" = [
          "",
        ]
        "resources" = [
          "configmaps",
        ]
        "verbs" = [
          "get",
          "list",
          "watch",
          "create",
          "update",
          "patch",
          "delete",
        ]
      },
      {
        "apiGroups" = [
          "coordination.k8s.io",
        ]
        "resources" = [
          "leases",
        ]
        "verbs" = [
          "get",
          "list",
          "watch",
          "create",
          "update",
          "patch",
          "delete",
        ]
      },
      {
        "apiGroups" = [
          "",
        ]
        "resources" = [
          "events",
        ]
        "verbs" = [
          "create",
          "patch",
        ]
      },
    ]
  }
}

resource "kubernetes_manifest" "clusterrole_db_operator_manager_role" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "ClusterRole"
    "metadata" = {
      "name" = "db-operator-manager-role"
    }
    "rules" = [
      {
        "apiGroups" = [
          "batch",
        ]
        "resources" = [
          "cronjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "batch",
        ]
        "resources" = [
          "jobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "",
        ]
        "resources" = [
          "configmaps",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "",
        ]
        "resources" = [
          "secrets",
        ]
        "verbs" = [
          "get",
          "list",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupcronjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupcronjobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupcronjobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupjobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backupjobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backuptargets",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backuptargets/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "backuptargets/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopycronjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopycronjobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopycronjobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopyjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopyjobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbcopyjobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbservers",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbservers/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "dbservers/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorecronjobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorecronjobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorecronjobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorejobs",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorejobs/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restorejobs/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restoretargets",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restoretargets/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "restoretargets/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "s3storages",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "s3storages/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "s3storages/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "users",
        ]
        "verbs" = [
          "create",
          "delete",
          "get",
          "list",
          "patch",
          "update",
          "watch",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "users/finalizers",
        ]
        "verbs" = [
          "update",
        ]
      },
      {
        "apiGroups" = [
          "db-operator.kubemaster.com",
        ]
        "resources" = [
          "users/status",
        ]
        "verbs" = [
          "get",
          "patch",
          "update",
        ]
      },
    ]
  }
}

resource "kubernetes_manifest" "clusterrole_db_operator_metrics_reader" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "ClusterRole"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "kube-rbac-proxy"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "metrics-reader"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "clusterrole"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-metrics-reader"
    }
    "rules" = [
      {
        "nonResourceURLs" = [
          "/metrics",
        ]
        "verbs" = [
          "get",
        ]
      },
    ]
  }
}

resource "kubernetes_manifest" "clusterrole_db_operator_proxy_role" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "ClusterRole"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "kube-rbac-proxy"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "proxy-role"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "clusterrole"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-proxy-role"
    }
    "rules" = [
      {
        "apiGroups" = [
          "authentication.k8s.io",
        ]
        "resources" = [
          "tokenreviews",
        ]
        "verbs" = [
          "create",
        ]
      },
      {
        "apiGroups" = [
          "authorization.k8s.io",
        ]
        "resources" = [
          "subjectaccessreviews",
        ]
        "verbs" = [
          "create",
        ]
      },
    ]
  }
}

resource "kubernetes_manifest" "rolebinding_db_operator_system_db_operator_leader_election_rolebinding" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "RoleBinding"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "rbac"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "leader-election-rolebinding"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "rolebinding"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-leader-election-rolebinding"
      "namespace" = "db-operator-system"
    }
    "roleRef" = {
      "apiGroup" = "rbac.authorization.k8s.io"
      "kind" = "Role"
      "name" = "db-operator-leader-election-role"
    }
    "subjects" = [
      {
        "kind" = "ServiceAccount"
        "name" = "db-operator-controller-manager"
        "namespace" = "db-operator-system"
      },
    ]
  }
}

resource "kubernetes_manifest" "clusterrolebinding_db_operator_manager_rolebinding" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "ClusterRoleBinding"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "rbac"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "manager-rolebinding"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "clusterrolebinding"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-manager-rolebinding"
    }
    "roleRef" = {
      "apiGroup" = "rbac.authorization.k8s.io"
      "kind" = "ClusterRole"
      "name" = "db-operator-manager-role"
    }
    "subjects" = [
      {
        "kind" = "ServiceAccount"
        "name" = "db-operator-controller-manager"
        "namespace" = "db-operator-system"
      },
    ]
  }
}

resource "kubernetes_manifest" "clusterrolebinding_db_operator_proxy_rolebinding" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "rbac.authorization.k8s.io/v1"
    "kind" = "ClusterRoleBinding"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "kube-rbac-proxy"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "proxy-rolebinding"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "clusterrolebinding"
        "app.kubernetes.io/part-of" = "db-operator"
      }
      "name" = "db-operator-proxy-rolebinding"
    }
    "roleRef" = {
      "apiGroup" = "rbac.authorization.k8s.io"
      "kind" = "ClusterRole"
      "name" = "db-operator-proxy-role"
    }
    "subjects" = [
      {
        "kind" = "ServiceAccount"
        "name" = "db-operator-controller-manager"
        "namespace" = "db-operator-system"
      },
    ]
  }
}

resource "kubernetes_manifest" "service_db_operator_system_db_operator_controller_manager_metrics_service" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "v1"
    "kind" = "Service"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "kube-rbac-proxy"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "controller-manager-metrics-service"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "service"
        "app.kubernetes.io/part-of" = "db-operator"
        "control-plane" = "controller-manager"
      }
      "name" = "db-operator-controller-manager-metrics-service"
      "namespace" = "db-operator-system"
    }
    "spec" = {
      "ports" = [
        {
          "name" = "https"
          "port" = 8443
          "protocol" = "TCP"
          "targetPort" = "https"
        },
      ]
      "selector" = {
        "control-plane" = "controller-manager"
      }
    }
  }
}

resource "kubernetes_manifest" "deployment_db_operator_system_db_operator_controller_manager" {
  provider = kubernetes-alpha

  manifest = {
    "apiVersion" = "apps/v1"
    "kind" = "Deployment"
    "metadata" = {
      "labels" = {
        "app.kubernetes.io/component" = "manager"
        "app.kubernetes.io/created-by" = "db-operator"
        "app.kubernetes.io/instance" = "controller-manager"
        "app.kubernetes.io/managed-by" = "kustomize"
        "app.kubernetes.io/name" = "deployment"
        "app.kubernetes.io/part-of" = "db-operator"
        "control-plane" = "controller-manager"
      }
      "name" = "db-operator-controller-manager"
      "namespace" = "db-operator-system"
    }
    "spec" = {
      "replicas" = 1
      "selector" = {
        "matchLabels" = {
          "control-plane" = "controller-manager"
        }
      }
      "template" = {
        "metadata" = {
          "annotations" = {
            "kubectl.kubernetes.io/default-container" = "manager"
          }
          "labels" = {
            "control-plane" = "controller-manager"
          }
        }
        "spec" = {
          "affinity" = {
            "nodeAffinity" = {
              "requiredDuringSchedulingIgnoredDuringExecution" = {
                "nodeSelectorTerms" = [
                  {
                    "matchExpressions" = [
                      {
                        "key" = "kubernetes.io/arch"
                        "operator" = "In"
                        "values" = [
                          "amd64",
                          "arm64",
                          "ppc64le",
                          "s390x",
                        ]
                      },
                      {
                        "key" = "kubernetes.io/os"
                        "operator" = "In"
                        "values" = [
                          "linux",
                        ]
                      },
                    ]
                  },
                ]
              }
            }
          }
          "containers" = [
            {
              "args" = [
                "--secure-listen-address=0.0.0.0:8443",
                "--upstream=http://127.0.0.1:8080/",
                "--logtostderr=true",
                "--v=0",
              ]
              "image" = "gcr.io/kubebuilder/kube-rbac-proxy:v0.13.0"
              "name" = "kube-rbac-proxy"
              "ports" = [
                {
                  "containerPort" = 8443
                  "name" = "https"
                  "protocol" = "TCP"
                },
              ]
              "resources" = {
                "limits" = {
                  "cpu" = "500m"
                  "memory" = "128Mi"
                }
                "requests" = {
                  "cpu" = "5m"
                  "memory" = "64Mi"
                }
              }
              "securityContext" = {
                "allowPrivilegeEscalation" = false
                "capabilities" = {
                  "drop" = [
                    "ALL",
                  ]
                }
              }
            },
            {
              "args" = [
                "--health-probe-bind-address=:8081",
                "--metrics-bind-address=127.0.0.1:8080",
                "--leader-elect",
              ]
              "command" = [
                "/manager",
              ]
              "image" = var.image
              "livenessProbe" = {
                "httpGet" = {
                  "path" = "/healthz"
                  "port" = 8081
                }
                "initialDelaySeconds" = 15
                "periodSeconds" = 20
              }
              "name" = "manager"
              "readinessProbe" = {
                "httpGet" = {
                  "path" = "/readyz"
                  "port" = 8081
                }
                "initialDelaySeconds" = 5
                "periodSeconds" = 10
              }
              "resources" = {
                "limits" = {
                  "cpu" = "500m"
                  "memory" = "128Mi"
                }
                "requests" = {
                  "cpu" = "10m"
                  "memory" = "64Mi"
                }
              }
              "securityContext" = {
                "allowPrivilegeEscalation" = false
                "capabilities" = {
                  "drop" = [
                    "ALL",
                  ]
                }
              }
            },
          ]
          "securityContext" = {
            "runAsNonRoot" = true
          }
          "serviceAccountName" = "db-operator-controller-manager"
          "terminationGracePeriodSeconds" = 10
        }
      }
    }
  }
}

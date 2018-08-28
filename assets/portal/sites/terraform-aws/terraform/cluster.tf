provider "aws" {
    access_key = "${var.access_key}"
    secret_key = "${var.secret_key}"
    region = "${var.region}"
}

module "kubernetes" {
    source = "git@github.com:gravitational/terra.git//aws/kubernetes"

    vpc_cidr_block = "10.0.0.0/16"
    subnet_cidr_block = "10.0.0.0/24"

    access_key = "${var.access_key}"
    secret_key = "${var.secret_key}"

    region = "${var.region}"
    az1 = "${var.az1}"
    az2 = "${var.az2}"

    cluster_name = "${var.cluster_name}"
    user_data_file = "${var.user_data_file}"
    ami = "${var.ami}"

    data_vol_device_name = "${var.data_vol_device_name}"
    data_vol_mount_point = "${var.data_vol_mount_point}"
    data_vol_size_gb = "${var.data_vol_size_gb}"
}

# TWO subnets for Postrgres in different availability zones
resource "aws_subnet" "postgres1" {
    vpc_id = "${module.kubernetes.vpc_id}"
    cidr_block = "10.0.1.0/24"
    availability_zone = "${var.az1}"

    tags {
        KubernetesClusterDB = "${var.cluster_name}"
    }
}

resource "aws_subnet" "postgres2" {
    vpc_id = "${module.kubernetes.vpc_id}"
    cidr_block = "10.0.2.0/24"
    availability_zone = "${var.az2}"
    
    tags {
        KubernetesClusterDB = "${var.cluster_name}"
    }
}

# Associate subnets and the routing table explicitly
resource "aws_route_table_association" "postgres1" {
    subnet_id = "${aws_subnet.postgres1.id}"
    route_table_id = "${module.kubernetes.route_table_id}"
}

resource "aws_route_table_association" "postgres2" {
    subnet_id = "${aws_subnet.postgres2.id}"
    route_table_id = "${module.kubernetes.route_table_id}"
}

# create a subnet group for Postgres
# for RDS, postgres should have a subnet group
# with two subnets in different AZ in the region
resource "aws_db_subnet_group" "postgres" {
    name = "${var.cluster_name}-postgres"    
    description = "Postgres RDB subnet group"
    subnet_ids = ["${aws_subnet.postgres1.id}", "${aws_subnet.postgres2.id}"]
}

# create RDB postgres instance
resource "aws_db_instance" "postgres1" {
    identifier = "${var.cluster_name}-postgres1"
    allocated_storage = "${var.pg_storage_gb}"
    engine = "postgres"
    engine_version = "9.4.1"
    instance_class = "${var.pg_instance}"
    name = "${var.pg_db}"
    username = "${var.pg_user}"
    password = "${var.pg_pass}"
    db_subnet_group_name = "${aws_db_subnet_group.postgres.name}"
    multi_az="true"
    vpc_security_group_ids = ["${module.kubernetes.security_group_id}"]
}

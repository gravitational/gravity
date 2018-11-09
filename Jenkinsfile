#!/usr/bin/env groovy
def propagateParamsToEnv() {
  for (param in params) {
    if (env."${param.key}" == null) {
      env."${param.key}" = param.value
    }
  }
}

properties([
  disableConcurrentBuilds(),
  parameters([
    choice(choices: ["run", "skip"].join("\n"),
           defaultValue: 'run',
           description: 'Run or skip robotest system wide tests.',
           name: 'RUN_ROBOTEST'),
    choice(choices: ["true", "false"].join("\n"),
           defaultValue: 'true',
           description: 'Destroy all VMs on success.',
           name: 'DESTROY_ON_SUCCESS'),
    choice(choices: ["true", "false"].join("\n"),
           defaultValue: 'true',
           description: 'Destroy all VMs on failure.',
           name: 'DESTROY_ON_FAILURE'),
    choice(choices: ["true", "false"].join("\n"),
           defaultValue: 'true',
           description: 'Abort all tests upon first failure.',
           name: 'FAIL_FAST'),
    choice(choices: ["gce"].join("\n"),
           defaultValue: 'gce',
           description: 'Cloud provider to deploy to.',
           name: 'DEPLOY_TO'),
    string(name: 'PARALLEL_TESTS',
           defaultValue: '4',
           description: 'Number of parallel tests to run.'),
    string(name: 'REPEAT_TESTS',
           defaultValue: '1',
           description: 'How many times to repeat each test.'),
    string(name: 'ROBOTEST_VERSION',
           defaultValue: 'dima',
           description: 'Robotest tag to use.'),
  ]),
])

timestamps {
node {
  stage('checkout') {
    checkout scm
    sh "git submodule update --init --recursive"
    sh "sudo git clean -ffdx" // supply -f flag twice to force-remove untracked dirs with .git subdirs (e.g. submodules)
  }
  stage('params') {
    echo "${params}"
    propagateParamsToEnv()
  }
  stage('clean') {
    sh "make -C e clean"
  }
  stage('build-gravity') {
    withCredentials([
    [$class: 'SSHUserPrivateKeyBinding', credentialsId: '08267d86-0b3a-4101-841e-0036bf780b11', keyFileVariable: 'GITHUB_SSH_KEY'],
    [$class: 'UsernamePasswordMultiBinding', credentialsId: 'jenkins-aws-s3', usernameVariable: 'AWS_ACCESS_KEY', passwordVariable: 'AWS_SECRET_KEY'],
    ]) {
      sh 'make -C e production telekube opscenter'
    }
  }
  stage('build-and-test') {
    parallel (
    build : {
      withCredentials([
      [$class: 'SSHUserPrivateKeyBinding', credentialsId: '08267d86-0b3a-4101-841e-0036bf780b11', keyFileVariable: 'GITHUB_SSH_KEY']]) {
        sh 'make test && make -C e test'
      }
    },
    robotest : {
      if (params.RUN_ROBOTEST == 'run') {
        withCredentials([
            [$class: 'UsernamePasswordMultiBinding', credentialsId: 'jenkins-aws-s3', usernameVariable: 'AWS_ACCESS_KEY', passwordVariable: 'AWS_SECRET_KEY'],
            [$class: 'StringBinding', credentialsId: 'GET_GRAVITATIONAL_IO_APIKEY', variable: 'GET_GRAVITATIONAL_IO_APIKEY'],
            [$class: 'FileBinding', credentialsId:'ROBOTEST_LOG_GOOGLE_APPLICATION_CREDENTIALS', variable: 'GOOGLE_APPLICATION_CREDENTIALS'],
            [$class: 'FileBinding', credentialsId:'OPS_SSH_KEY', variable: 'SSH_KEY'],
            [$class: 'FileBinding', credentialsId:'OPS_SSH_PUB', variable: 'SSH_PUB'],
            ]) {
              sh """
              make -C e robotest-run-suite \
                AWS_KEYPAIR=ops \
                AWS_REGION=us-east-1 \
                ROBOTEST_VERSION=$ROBOTEST_VERSION"""
        }
      }else {
        echo 'skipped system tests'
      }
    } )
  }
} }

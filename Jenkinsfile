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
    choice(choices: ["azure", "aws"].join("\n"),
           defaultValue: 'azure',
           description: 'Cloud provider to deploy to.',
           name: 'DEPLOY_TO'),
    string(name: 'PARALLEL_TESTS',
           defaultValue: '80',
           description: 'Number of parallel tests to run.'),
    string(name: 'REPEAT_TESTS',
           defaultValue: '1',
           description: 'How many times to repeat each test.'),
    string(name: 'ROBOTEST_VERSION',
           defaultValue: 'stable',
           description: 'Robotest tag to use.'),
  ]),
])

timestamps {
node {
  stage('checkout') {
    checkout scm
    sh "git submodule update --init"
    sh "sudo git clean -ffdx" // supply -f flag twice to force-remove untracked dirs with .git subdirs (e.g. submodules)
  }
  stage('params') {
    echo "${params}"
    propagateParamsToEnv()
  }
  stage('clean') {
    sh "make -C e clean"
  }
  stage('build-telekube') {
    withCredentials([
    [$class: 'SSHUserPrivateKeyBinding', credentialsId: '08267d86-0b3a-4101-841e-0036bf780b11', keyFileVariable: 'GITHUB_SSH_KEY']]) {
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
            [$class: 'UsernamePasswordMultiBinding', credentialsId: 'jenkins-aws', usernameVariable: 'AWS_ACCESS_KEY', passwordVariable: 'AWS_SECRET_KEY'],
            [$class: 'StringBinding', credentialsId: 'GET_GRAVITATIONAL_IO_APIKEY', variable: 'GET_GRAVITATIONAL_IO_APIKEY'],
            [$class: 'StringBinding', credentialsId: 'AZURE_SUBSCRIPTION_ID', variable: 'AZURE_SUBSCRIPTION_ID'],
            [$class: 'StringBinding', credentialsId: 'AZURE_TENANT_ID', variable: 'AZURE_TENANT_ID'],
            [$class: 'StringBinding', credentialsId: 'AZURE_CLIENT_SECRET', variable: 'AZURE_CLIENT_SECRET'],
            [$class: 'StringBinding', credentialsId: 'AZURE_CLIENT_ID', variable: 'AZURE_CLIENT_ID'],
            [$class: 'FileBinding', credentialsId:'ROBOTEST_LOG_GOOGLE_APPLICATION_CREDENTIALS', variable: 'GOOGLE_APPLICATION_CREDENTIALS'],
            [$class: 'FileBinding', credentialsId:'OPS_SSH_KEY', variable: 'SSH_KEY'],
            [$class: 'FileBinding', credentialsId:'OPS_SSH_PUB', variable: 'SSH_PUB'],
            ]) {
              sh """
              make -C e robotest-run-suite \
                AWS_KEYPAIR=ops \
                AWS_REGION=us-east-1 \
                AZURE_VM=Standard_F4s \
                ROBOTEST_VERSION=$ROBOTEST_VERSION"""
        }
      }else {
        echo 'skipped system tests'
      }
    } )
  }
} }

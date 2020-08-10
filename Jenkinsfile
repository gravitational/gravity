#!/usr/bin/env groovy
def propagateParamsToEnv() {
  for (param in params) {
    if (env."${param.key}" == null) {
      env."${param.key}" = param.value
    }
  }
}

// Define Robotest config that may be tweaked per job.
// This is needed for the Jenkins GitHub Branch Source Plugin
// which creases a unique Jenkins job for each pull request.
def setRobotestParameters() {
  properties([
    disableConcurrentBuilds(),
    parameters([
      // WARNING: changing parameters will not affect the next build, only the following one
      // see issue #1315 or https://stackoverflow.com/questions/46680573/ -- 2020-04 walt
      choice(choices: ["run", "skip"].join("\n"),
             // defaultValue is not applicable to choices. The first choice will be the default.
             description: 'Run or skip robotest system wide tests.',
             name: 'RUN_ROBOTEST'),
      choice(choices: ["true", "false"].join("\n"),
             description: 'Destroy all VMs on success.',
             name: 'DESTROY_ON_SUCCESS'),
      choice(choices: ["true", "false"].join("\n"),
             description: 'Destroy all VMs on failure.',
             name: 'DESTROY_ON_FAILURE'),
      choice(choices: ["true", "false"].join("\n"),
             description: 'Abort all tests upon first failure.',
             name: 'FAIL_FAST'),
    ]),
  ])
}

timestamps {
  node {
    stage('checkout') {
      checkout scm
      sh "git submodule update --init --recursive"
      sh "sudo git clean -ffdx" // supply -f flag twice to force-remove untracked dirs with .git subdirs (e.g. submodules)
    }
    stage('params') {
      // For try builds, we DO NOT want to overwrite parameters, as try builds
      // offer a superset of PR/nightly parameters, and the extra ones will be
      // lost when setRobotestParameters() is called -- 2020-04 walt
      echo "Jenkins Job Parameters:"
      for (param in params) { echo "${param}" }
      if (env.KEEP_PARAMETERS == 'true') {
        echo "KEEP_PARAMETERS detected. Ignoring Jenkins job parameters from Jenkinsfile."
      } else {
        echo "Overwriting Jenkins job parameters with parameters from Jenkinsfile."
        setRobotestParameters()
        propagateParamsToEnv()
      }
    }
    stage('clean') {
      sh "make -C e clean"
    }
    stage('build-gravity') {
      withCredentials([
      sshUserPrivateKey(credentialsId: '08267d86-0b3a-4101-841e-0036bf780b11', keyFileVariable: 'GITHUB_SSH_KEY'),
      usernamePassword(credentialsId: 'jenkins-aws-s3', usernameVariable: 'AWS_ACCESS_KEY_ID', passwordVariable: 'AWS_SECRET_ACCESS_KEY'),
      string(credentialsId:'GET_GRAVITATIONAL_IO_APIKEY', variable: 'GET_GRAVITATIONAL_IO_APIKEY'),
      ]) {
        sh 'make -C e production telekube-intermediate-upgrade opscenter'
      }
    }
  }
  throttle(['robotest']) {
    node {
      stage('build-and-test') {
        parallel (
        build : {
          withCredentials([
          sshUserPrivateKey(credentialsId: '08267d86-0b3a-4101-841e-0036bf780b11', keyFileVariable: 'GITHUB_SSH_KEY')
          ]) {
            sh 'make test && make -C e test'
          }
        },
        robotest : {
          if (env.RUN_ROBOTEST == 'run') {
            withCredentials([
                string(credentialsId: 'GET_GRAVITATIONAL_IO_APIKEY', variable: 'GET_GRAVITATIONAL_IO_APIKEY'),
                file(credentialsId:'ROBOTEST_LOG_GOOGLE_APPLICATION_CREDENTIALS', variable: 'GOOGLE_APPLICATION_CREDENTIALS'),
                file(credentialsId:'OPS_SSH_KEY', variable: 'SSH_KEY'),
                file(credentialsId:'OPS_SSH_PUB', variable: 'SSH_PUB'),
                string(credentialsId:'GET_GRAVITATIONAL_IO_APIKEY', variable: 'BUILD_APIKEY'),
                ]) {
                  sh 'make -C e robotest-run-suite'
            }
          }else {
            echo 'skipped system tests'
          }
        } )
      }
    }
  }
}

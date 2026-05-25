// =============================================================================
// Jenkinsfile — Declarative Pipeline with SonarQube Quality Gate
// + Discord notifications
// =============================================================================

pipeline {

    agent any

    triggers {
        pollSCM('* * * * *')
    }

    tools {
        'hudson.plugins.sonar.SonarRunnerInstallation' 'Sonarqube Scanner'
    }

    environment {
        IMAGE_TAG         = 'unknown'
        REGISTRY_URL      = "${env.REGISTRY_URL ?: 'docker.io'}"
        DOCKER_NAMESPACE  = "${env.DOCKER_NAMESPACE ?: 'ckyyy'}"
        DOCKERHUB_CRED_ID = 'dockerhub-credentials'
        GITHUB_CRED_ID    = 'github-credentials'
        DISCORD_WEBHOOK   = credentials('discord-webhook')
        GO_SERVICES       = 'services/api'
        GOTOOLCHAIN       = 'local'
        NEXTJS_SERVICES   = 'services/frontend'
        PYTHON_SERVICES   = 'services/rule-engine'
    }

    options {
        timestamps()
        disableConcurrentBuilds()
        timeout(time: 30, unit: 'MINUTES')
        buildDiscarder(logRotator(numToKeepStr: '20'))
    }

    stages {

        stage('Checkout') {
            steps {
                script {
                    checkout scm
                    env.IMAGE_TAG = sh(returnStdout: true, script: 'git rev-parse --short HEAD').trim()
                    env.GIT_BRANCH_NAME = sh(returnStdout: true, script: 'git symbolic-ref --short HEAD || git rev-parse --short HEAD').trim()
                    echo "Commit: ${env.IMAGE_TAG} | Branch: ${env.GIT_BRANCH_NAME}"
                }
            }
        }

        stage('Lint') {
            parallel {
                stage('Lint — Go') {
                    when { expression { fileExists(env.GO_SERVICES) } }
                    steps {
                        dir(env.GO_SERVICES) {
                            sh 'go vet ./...'
                        }
                    }
                }
                stage('Lint — Next.js') {
                    when { expression { fileExists(env.NEXTJS_SERVICES) } }
                    steps {
                        dir(env.NEXTJS_SERVICES) {
                            sh '''
                                npm ci --prefer-offline --loglevel=warn
                                npx eslint . --ext .js,.jsx,.ts,.tsx --max-warnings 0
                            '''
                        }
                    }
                }
                stage('Lint — Python') {
                    when { expression { fileExists(env.PYTHON_SERVICES) } }
                    steps {
                        dir(env.PYTHON_SERVICES) {
                            sh '''
                               sudo apt-get install -y python3-venv
                               ./venv/bin/pip install flake8
                               ./venv/bin/flake8 .
                            '''
                        }
                    }
                }
            }
        }

        stage('Test') {
            parallel {
                stage('Test — Go') {
                    when { expression { fileExists(env.GO_SERVICES) } }
                    steps {
                        dir(env.GO_SERVICES) {
                            sh '''
                                mkdir -p test-results
                                go test -v ./... -coverprofile=coverage.out 2>&1 | tee test-results/go-test.txt
                            '''
                        }
                    }
                }
                stage('Test — Next.js') {
                    when { expression { fileExists(env.NEXTJS_SERVICES) } }
                    steps {
                        dir(env.NEXTJS_SERVICES) {
                            sh 'npx jest --ci --coverage --passWithNoTests'
                        }
                    }
                }
                stage('Test — Python') {
                    when { expression { fileExists(env.PYTHON_SERVICES) } }
                    steps {
                        dir(env.PYTHON_SERVICES) {
                            sh '''
                                python3 -m pip install --quiet -r requirements.txt pytest pytest-cov
                                mkdir -p test-results
                                pytest --tb=short \
                                       --junitxml=test-results/pytest.xml \
                                       --cov=. --cov-report=xml:test-results/coverage.xml
                            '''
                        }
                    }
                    post {
                        always {
                            junit allowEmptyResults: true,
                                  testResults: "${env.PYTHON_SERVICES}/test-results/pytest.xml"
                        }
                    }
                }
            }
        }

        stage('SonarQube Analysis') {
            steps {
                withSonarQubeEnv('sonarqube') {
                    sh """
                        export PATH=\$PATH:\$SONAR_RUNNER_HOME/bin
                        sonar-scanner \
                          -Dsonar.projectKey=final-project-ncc-kel3 \
                          -Dsonar.projectName='Final Project NCC Kel3' \
                          -Dsonar.sources=. \
                          -Dsonar.exclusions=**/node_modules/**,**/.git/**,**/vendor/**,**/__pycache__/** \
                          -Dsonar.go.coverage.reportPaths=${GO_SERVICES}/coverage.out \
                          -Dsonar.python.coverage.reportPaths=${PYTHON_SERVICES}/test-results/coverage.xml \
                          -Dsonar.javascript.lcov.reportPaths=${NEXTJS_SERVICES}/coverage/lcov.info \
                          -Dsonar.scm.revision=${IMAGE_TAG}
                    """
                }
            }
        }

        stage('Quality Gate') {
            steps {
                timeout(time: 5, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: true
                }
            }
        }

        stage('Build & Push Images') {
            when {
                anyOf {
                    branch 'main'
                    changeRequest()
                }
            }
            steps {
                script {
                    withCredentials([usernamePassword(
                        credentialsId: env.DOCKERHUB_CRED_ID,
                        usernameVariable: 'DOCKER_USER',
                        passwordVariable: 'DOCKER_PASS'
                    )]) {
                        sh 'echo "$DOCKER_PASS" | docker login -u "$DOCKER_USER" --password-stdin https://docker.io'

                        def services = [
                            [dir: env.GO_SERVICES,     name: 'siem-api'],
                            [dir: env.NEXTJS_SERVICES, name: 'siem-frontend'],
                            [dir: env.PYTHON_SERVICES, name: 'siem-rule-engine'],
                        ]

                        services.each { svc ->
                            if (fileExists("${svc.dir}/Dockerfile")) {
                                def tag = "${env.DOCKER_NAMESPACE}/${svc.name}:${env.IMAGE_TAG}"
                                sh "docker build -t ${tag} --pull ${svc.dir}"
                                sh "docker push ${tag}"
                                if (env.BRANCH_NAME == 'main') {
                                    def latestTag = "${env.DOCKER_NAMESPACE}/${svc.name}:latest"
                                    sh "docker tag ${tag} ${latestTag}"
                                    sh "docker push ${latestTag}"
                                }
                            }
                        }

                        sh 'docker logout https://docker.io || true'
                    }
                }
            }
        }

    } // end stages

    post {
        always {
            script {
                sh 'docker image prune -f || true'
                cleanWs()
            }
        }
        success {
            script {
                def msg = """{"embeds":[{"title":"✅ Build Passed","color":3066993,"fields":[{"name":"Job","value":"${env.JOB_NAME}","inline":true},{"name":"Build","value":"#${env.BUILD_NUMBER}","inline":true},{"name":"Commit","value":"${env.IMAGE_TAG}","inline":true},{"name":"Branch","value":"${env.BRANCH_NAME}","inline":true},{"name":"Console","value":"[View Output](${env.BUILD_URL}console)","inline":false}]}]}"""
                sh """curl -s -X POST -H 'Content-Type: application/json' -d '${msg}' ${DISCORD_WEBHOOK}"""
            }
        }
        failure {
            script {
                def msg = """{"embeds":[{"title":"❌ Build Failed","color":15158332,"fields":[{"name":"Job","value":"${env.JOB_NAME}","inline":true},{"name":"Build","value":"#${env.BUILD_NUMBER}","inline":true},{"name":"Commit","value":"${env.IMAGE_TAG}","inline":true},{"name":"Branch","value":"${env.BRANCH_NAME}","inline":true},{"name":"Console","value":"[View Output](${env.BUILD_URL}console)","inline":false}]}]}"""
                sh """curl -s -X POST -H 'Content-Type: application/json' -d '${msg}' ${DISCORD_WEBHOOK}"""
            }
        }
        unstable {
            script {
                def msg = """{"embeds":[{"title":"⚠️ Build Unstable","color":16776960,"fields":[{"name":"Job","value":"${env.JOB_NAME}","inline":true},{"name":"Build","value":"#${env.BUILD_NUMBER}","inline":true},{"name":"Branch","value":"${env.BRANCH_NAME}","inline":true}]}]}"""
                sh """curl -s -X POST -H 'Content-Type: application/json' -d '${msg}' ${DISCORD_WEBHOOK}"""
            }
        }
    }

} // end pipeline

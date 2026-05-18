pipeline {
    agent any

    options {
        timestamps()
        disableConcurrentBuilds()
    }

    triggers {
        githubPush()
    }

    parameters {
        string(name: 'GIT_BRANCH', defaultValue: 'day-2', description: 'Git branch to checkout')
        string(name: 'GIT_URL', defaultValue: 'https://github.com/irfansandyy/tugas-ncc-irfansandy.git', description: 'Repository URL')
        string(name: 'GIT_CREDENTIALS_ID', defaultValue: '', description: 'Optional Jenkins credentials ID for private repos')
    }

    environment {
        SONARQUBE_ENV = 'SonarQube'
        PROJECT_KEY   = 'tugas-ncc-irfansandy-backend'
        PROJECT_NAME  = 'tugas-ncc-irfansandy-fullstack'
        GO_DIR        = 'backend'
        FE_DIR        = 'frontend'
        GOFLAGS       = '-buildvcs=false'
        SCANNER_HOME  = tool 'SonarQube Scanner'
    }

    stages {
        stage('Checkout') {
            steps {
                script {
                    try {
                        deleteDir()
                    } catch (Exception cleanupErr) {
                        echo "Workspace cleanup failed, fixing ownership/permissions and retrying deleteDir()"
                        sh '''
                            set -e
                            docker run --rm -u root -v "${WORKSPACE}:/workspace" alpine:3.21 \
                              sh -c 'chown -R 1000:1000 /workspace || true; chmod -R u+rwX /workspace || true'
                        '''
                        deleteDir()
                    }

                    if (params.GIT_CREDENTIALS_ID?.trim()) {
                        git branch: params.GIT_BRANCH,
                            url: params.GIT_URL,
                            credentialsId: params.GIT_CREDENTIALS_ID
                    } else {
                        git branch: params.GIT_BRANCH,
                            url: params.GIT_URL
                    }
                }
            }
        }

        stage('Setup') {
            agent {
                docker {
                    image 'golang:1.23-bookworm'
                    args '''-e HOME=/tmp \
                            -e GOCACHE=/tmp/go-cache \
                            -e GOPATH=/tmp/go \
                            -v /var/jenkins_home/tools:/var/jenkins_home/tools'''
                    reuseNode true
                }
            }
            steps {
                sh '''
                    set -e
                    git config --global --add safe.directory "${WORKSPACE}"
                    cd "${GO_DIR}"
                    go version
                    go mod download
                '''
            }
        }

        stage('Backend Build & Test') {
            parallel {
                stage('Build') {
                    agent {
                        docker {
                            image 'golang:1.23-bookworm'
                            args '''-e HOME=/tmp \
                                    -e GOCACHE=/tmp/go-cache \
                                    -e GOPATH=/tmp/go'''
                            reuseNode true
                        }
                    }
                    steps {
                        sh '''
                            set -e
                            cd "${GO_DIR}"
                            go build -v ./...
                        '''
                    }
                }

                stage('Test') {
                    agent {
                        docker {
                            image 'golang:1.23-bookworm'
                            args '''-e HOME=/tmp \
                                    -e GOCACHE=/tmp/go-cache \
                                    -e GOPATH=/tmp/go'''
                            reuseNode true
                        }
                    }
                    steps {
                        sh '''
                            set -e
                            cd "${GO_DIR}"
                            go test ./... -v -coverprofile=coverage.out
                        '''
                    }
                }
            }
        }

        stage('Frontend Install') {
            agent {
                docker {
                    image 'node:22-bookworm'
                    args '-e HOME=/tmp'
                    reuseNode true
                }
            }
            steps {
                sh '''
                    set -e
                    cd "${FE_DIR}"
                    if [ -f package-lock.json ]; then
                        npm ci
                    else
                        npm install
                    fi
                '''
            }
        }

        stage('Frontend Lint') {
            agent {
                docker {
                    image 'node:22-bookworm'
                    args '-e HOME=/tmp'
                    reuseNode true
                }
            }
            steps {
                sh '''
                    set -e
                    cd "${FE_DIR}"
                    if npx --yes next --help 2>&1 | grep -Eq '(^|[[:space:]])lint([[:space:]]|$)'; then
                        npm run lint
                    else
                        echo "next lint is not available on this Next.js version, skipping lint stage."
                    fi
                '''
            }
        }

        stage('Frontend Build') {
            agent {
                docker {
                    image 'node:22-bookworm'
                    args '-e HOME=/tmp'
                    reuseNode true
                }
            }
            steps {
                sh '''
                    set -e
                    cd "${FE_DIR}"
                    npm run build
                '''
            }
        }

        stage('SonarQube Analysis') {
            steps {
                withSonarQubeEnv("${SONARQUBE_ENV}") {
                    sh '''
                        set -e
                        "${SCANNER_HOME}"/bin/sonar-scanner \
                          -Dsonar.projectKey="${PROJECT_KEY}" \
                          -Dsonar.projectName="${PROJECT_NAME}" \
                          -Dsonar.sources=backend,frontend \
                          -Dsonar.tests=backend,frontend \
                          -Dsonar.test.inclusions=backend/**/*_test.go,frontend/**/*.test.js,frontend/**/*.test.ts,frontend/**/*.spec.js,frontend/**/*.spec.ts \
                          -Dsonar.exclusions=frontend/.next/**,frontend/node_modules/** \
                          -Dsonar.go.coverage.reportPaths=backend/coverage.out
                    '''
                }
            }
        }

        stage('Quality Gate') {
            steps {
                timeout(time: 20, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: true
                }
            }
        }
    }

    post {
        success {
            echo 'Pipeline sukses'
        }
        failure {
            echo 'Pipeline gagal'
        }
        always {
            archiveArtifacts artifacts: 'backend/coverage.out', allowEmptyArchive: true
            archiveArtifacts artifacts: 'frontend/.next/**', allowEmptyArchive: true
        }
    }
}

// ============================================================
// Jenkinsfile
// Generates test coverage for Go services and the Next.js
// frontend, then feeds the reports into SonarQube.
// ============================================================

pipeline {
    agent any

    environment {
        // Override in Jenkins → Manage Jenkins → Configure System
        // or add credentials with these IDs.
        SONAR_HOST_URL  = 'http://sonarqube:9000/sonarqube'
        SONAR_TOKEN     = credentials('sonarqube-token')   // Secret-text credential
    }

    // tools {
        // Name must match what you configured in Manage Jenkins → Tools
        // nodejs 'NodeJS-20'
    // }

    stages {

        // ------------------------------------------------------------------
        // 1. Checkout
        // ------------------------------------------------------------------
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        // ------------------------------------------------------------------
        // 2. Go – install dependencies for every service
        // ------------------------------------------------------------------
        stage('Go: Install Dependencies') {
            steps {
                sh '''
                    echo "=== services/api ==="
                    cd services/api
                    go mod tidy
                    cd ../..

                    echo "=== services/log-collector ==="
                    cd services/log-collector
                    go mod tidy
                    cd ../..

                    echo "=== services/log-parser ==="
                    cd services/log-parser
                    go mod tidy
                    cd ../..
                '''
            }
        }

        // ------------------------------------------------------------------
        // 3. Go – run tests + generate coverage profiles
        //    Each service writes its own coverage.out so SonarQube can
        //    attribute coverage to the right files.
        // ------------------------------------------------------------------
        stage('Go: Test & Coverage') {
            steps {
                sh '''
                    echo "=== services/api ==="
                    cd services/api
                    go test -v -coverprofile=coverage.out -covermode=atomic ./...
                    go tool cover -func=coverage.out   # prints summary to console
                    cd ../..

                    echo "=== services/log-collector ==="
                    cd services/log-collector
                    go test -v -coverprofile=coverage.out -covermode=atomic ./...
                    go tool cover -func=coverage.out
                    cd ../..

                    echo "=== services/log-parser ==="
                    cd services/log-parser
                    go test -v -coverprofile=coverage.out -covermode=atomic ./...
                    go tool cover -func=coverage.out
                    cd ../..
                '''
            }
            post {
                always {
                    // Archive the raw profiles so they are available as
                    // build artefacts even if later stages fail.
                    archiveArtifacts artifacts: 'services/**/coverage.out', allowEmptyArchive: true
                }
            }
        }

        // ------------------------------------------------------------------
        // 4. Frontend – install dependencies
        // ------------------------------------------------------------------
        stage('Frontend: Install Dependencies') {
            steps {
                dir('frontend') {
                    sh 'npm ci'
                }
            }
        }

        // ------------------------------------------------------------------
        // 5. Frontend – run Jest with coverage (outputs lcov.info)
        //    jest.config.js must include:
        //      coverageReporters: ['lcov', 'text'],
        //      collectCoverageFrom: ['lib/**/*.ts', 'components/**/*.tsx',
        //                            'app/**/*.tsx']
        // ------------------------------------------------------------------
        stage('Frontend: Test & Coverage') {
            steps {
                dir('frontend') {
                    sh 'npm test -- --coverage --watchAll=false --ci'
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: 'frontend/coverage/**', allowEmptyArchive: true
                    // Publish JUnit-compatible output if jest-junit reporter is configured
                    junit allowEmptyResults: true, testResults: 'frontend/coverage/junit.xml'
                }
            }
        }

        // ------------------------------------------------------------------
        // 6. SonarQube Analysis
        //    Uses the SonarQube Scanner tool configured in
        //    Manage Jenkins → Tools → SonarQube Scanner (name: "SonarQube Scanner")
        //    and the server configured in
        //    Manage Jenkins → System → SonarQube servers (name: "SonarQube")
        // ------------------------------------------------------------------
        stage('SonarQube Analysis') {
            steps {
                withSonarQubeEnv('SonarQube') {
                    script {
                        def scannerHome = tool 'SonarQube Scanner'
                        sh """
                            ${scannerHome}/bin/sonar-scanner \\
                              -Dsonar.projectKey=final-project-ncc-kel3 \\
                              -Dsonar.projectName='Final Project NCC Kel3' \\
                              -Dsonar.sources=services,frontend \\
                              -Dsonar.tests=services,frontend \\
                              -Dsonar.test.inclusions='**/*_test.go,**/*.test.ts,**/*.test.tsx' \\
                              -Dsonar.exclusions='**/node_modules/**,**/.next/**,**/vendor/**,**/coverage/**' \\
                              -Dsonar.go.coverage.reportPaths=services/api/coverage.out,services/log-collector/coverage.out,services/log-parser/coverage.out \\
                              -Dsonar.javascript.lcov.reportPaths=frontend/coverage/lcov.info \\
                              -Dsonar.host.url=${SONAR_HOST_URL} \\
                              -Dsonar.login=${SONAR_TOKEN}
                        """
                    }
                }
            }
        }

        // ------------------------------------------------------------------
        // 7. Quality Gate – fail the build if gate is not passed
        // ------------------------------------------------------------------
        stage('Quality Gate') {
            steps {
                timeout(time: 5, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: true
                }
            }
        }

        // ------------------------------------------------------------------
        // 8. Build Docker images (only on main branch after gate passes)
        // ------------------------------------------------------------------
        stage('Build & Push Docker Images') {
            when {
                branch 'main'
            }
            steps {
                sh 'docker compose --env-file .env build'
            }
        }
    }

    // ------------------------------------------------------------------
    // Post-pipeline notifications
    // ------------------------------------------------------------------
    post {
        success {
            echo 'Pipeline passed – all tests green and quality gate met.'
        }
        failure {
            echo 'Pipeline FAILED. Check test output and SonarQube for details.'
        }
        always {
            cleanWs()
        }
    }
}

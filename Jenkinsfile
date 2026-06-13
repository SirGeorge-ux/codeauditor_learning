pipeline {
    agent any

    environment {
        GO_VERSION = '1.23'
        PNPM_VERSION = '9'
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Setup') {
            parallel {
                stage('Go Dependencies') {
                    steps {
                        dir('backend') {
                            sh 'go mod download'
                        }
                    }
                }
                stage('Frontend Dependencies') {
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate'
                            sh 'pnpm install --frozen-lockfile'
                        }
                    }
                }
            }
        }

        stage('Lint') {
            parallel {
                stage('Go Lint') {
                    steps {
                        dir('backend') {
                            sh 'golangci-lint run ./... || true'
                        }
                    }
                }
                stage('Frontend Lint') {
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'pnpm lint || true'
                        }
                    }
                }
            }
        }

        stage('Test') {
            parallel {
                stage('Go Tests') {
                    steps {
                        dir('backend') {
                            sh 'go test -count=1 -timeout=120s ./internal/...'
                        }
                    }
                }
                stage('Angular Tests') {
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'pnpm test -- --watch=false'
                        }
                    }
                }
            }
        }

        stage('Build') {
            parallel {
                stage('Go Build') {
                    steps {
                        dir('backend') {
                            sh 'go build -o api ./cmd/api/'
                        }
                    }
                }
                stage('Angular Build') {
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'pnpm build'
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            cleanWs()
        }
        success {
            echo 'CI passed — all tests and builds successful.'
        }
        failure {
            echo 'CI failed — check stage logs for details.'
        }
    }
}

pipeline {
    agent none

    environment {
        PNPM_VERSION = '9'
    }

    stages {
        stage('Checkout') {
            agent any
            steps {
                checkout scm
            }
        }

        stage('Setup') {
            parallel {
                stage('Go Dependencies') {
                    agent { docker { image 'golang:1.23-alpine' } }
                    steps {
                        dir('backend') {
                            sh 'go mod download'
                        }
                    }
                }
                stage('Frontend Dependencies') {
                    agent { docker { image 'node:22-alpine' } }
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
                    agent { docker { image 'golangci/golangci-lint:v1.64-alpine' } }
                    steps {
                        dir('backend') {
                            sh 'golangci-lint run ./... || true'
                        }
                    }
                }
                stage('Frontend Lint') {
                    agent { docker { image 'node:22-alpine' } }
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate'
                            sh 'pnpm install --frozen-lockfile && pnpm lint || true'
                        }
                    }
                }
            }
        }

        stage('Test') {
            parallel {
                stage('Go Tests') {
                    agent { docker { image 'golang:1.23-alpine' } }
                    steps {
                        dir('backend') {
                            sh 'go test -count=1 -timeout=120s ./internal/...'
                        }
                    }
                }
                stage('Angular Tests') {
                    agent { docker { image 'node:22-alpine' } }
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate'
                            sh 'pnpm install --frozen-lockfile && pnpm test -- --watch=false'
                        }
                    }
                }
            }
        }

        stage('Build') {
            parallel {
                stage('Go Build') {
                    agent { docker { image 'golang:1.23-alpine' } }
                    steps {
                        dir('backend') {
                            sh 'go build -o api ./cmd/api/'
                        }
                    }
                }
                stage('Angular Build') {
                    agent { docker { image 'node:22-alpine' } }
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate'
                            sh 'pnpm install --frozen-lockfile && pnpm build'
                        }
                    }
                }
            }
        }
    }

    post {
        success {
            echo 'CI passed — all tests and builds successful.'
        }
        failure {
            echo 'CI failed — check stage logs for details.'
        }
    }
}

pipeline {
    agent any

    environment {
        CI = 'true'
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Install Tools') {
            steps {
                sh """
                    apt-get update -qq
                    apt-get install -y -qq golang-go nodejs npm \\
                        libglib2.0-0 libnss3 libnspr4 libatk1.0-0t64 \\
                        libatk-bridge2.0-0t64 libcups2t64 libdrm2 \\
                        libxkbcommon0 libxcomposite1 libxdamage1 \\
                        libxfixes3 libxrandr2 libgbm1 libpango-1.0-0 \\
                        libcairo2 libasound2t64
                    npm install -g pnpm@9
                    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v1.64.8
                    npx playwright install chromium
                """
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
                            sh 'golangci-lint run ./...'
                        }
                    }
                }
                stage('Frontend Lint') {
                    steps {
                        dir('frontend/codeauditor') {
                            sh 'pnpm lint'
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
        stage('E2E') {
            steps {
                dir('frontend/codeauditor') {
                    sh 'npx playwright test --reporter=list'
                }
            }
        }
    }

    post {
        success {
            echo 'CI passed'
        }
        failure {
            echo 'CI failed'
        }
    }
}

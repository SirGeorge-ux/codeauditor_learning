pipeline {
    agent any

    environment {
        GO_VERSION = '1.23.4'
        GOROOT = "${WORKSPACE}/.goroot"
        GOPATH = "${WORKSPACE}/.go"
        PATH = "${GOROOT}/bin:${GOPATH}/bin:${PATH}"
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Install Tools') {
            steps {
                script {
                    // Install Go via tarball (fast, self-contained)
                    if (sh(script: 'go version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            mkdir -p ${GOROOT} ${GOPATH}/bin
                            curl -sL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C ${GOROOT} --strip-components=1 -xzf -
                        """
                    }

                    // Install Node.js 22 via NodeSource (reliable PATH setup + corepack)
                    if (sh(script: 'node --version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
                            apt-get install -y nodejs
                        """
                    }

                    // Enable corepack + install pnpm
                    sh 'corepack enable && corepack prepare pnpm@9 --activate'
                }
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
        success {
            echo 'CI passed — all tests and builds successful.'
        }
        failure {
            echo 'CI failed — check stage logs for details.'
        }
    }
}

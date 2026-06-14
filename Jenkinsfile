pipeline {
    agent any

    environment {
        GO_VERSION = '1.23.4'
        NODE_VERSION = '22'
        PNPM_VERSION = '9'
        GOPATH = "${WORKSPACE}/.go"
        GOROOT = "${WORKSPACE}/.goroot"
        PNPM_HOME = "${WORKSPACE}/.pnpm"
        PATH = "${GOROOT}/bin:${GOPATH}/bin:${PNPM_HOME}:${PATH}"
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
                    // Install Go
                    if (sh(script: 'go version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            mkdir -p ${GOROOT}
                            wget -qO- https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C ${GOROOT} --strip-components=1 -xzf -
                        """
                    }
                    // Install Node.js
                    if (sh(script: 'node --version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            mkdir -p /tmp/node
                            wget -qO- https://nodejs.org/dist/v${NODE_VERSION}.0.0/node-v${NODE_VERSION}.0.0-linux-x64.tar.xz | tar -C /tmp/node --strip-components=1 -xJf -
                            mkdir -p ${GOPATH}/bin
                            cp /tmp/node/bin/node ${GOPATH}/bin/
                            cp /tmp/node/bin/npm ${GOPATH}/bin/
                            cp /tmp/node/bin/npx ${GOPATH}/bin/
                            ln -sf ../lib/node_modules/corepack/dist/corepack.js /tmp/node/bin/corepack 2>/dev/null || true
                            PATH="${PATH}:/tmp/node/bin" npx corepack enable
                        """
                    }
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
                            sh 'corepack enable 2>/dev/null; corepack prepare pnpm@${PNPM_VERSION} --activate 2>/dev/null'
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

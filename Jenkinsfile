pipeline {
    agent any

    environment {
        GO_VERSION = '1.23.4'
        NODE_VERSION = '22.11.0'
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
                    sh 'mkdir -p ${GOPATH}/bin'
                    // Install Go if not present
                    if (sh(script: 'go version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            mkdir -p ${GOROOT} ${GOPATH}/bin
                            curl -sL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C ${GOROOT} --strip-components=1 -xzf -
                        """
                    }
                    // Install Node.js if not present
                    if (sh(script: 'node --version 2>/dev/null || true', returnStdout: true).trim() == '') {
                        sh """
                            apt-get update -qq && apt-get install -y -qq xz-utils 2>/dev/null || true
                            curl -sL https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz -o /tmp/node.tar.xz
                            tar -C /tmp -xJf /tmp/node.tar.xz
                            mkdir -p ${GOPATH}/bin
                            cp /tmp/node-v${NODE_VERSION}-linux-x64/bin/node ${GOPATH}/bin/
                            cp /tmp/node-v${NODE_VERSION}-linux-x64/bin/npm ${GOPATH}/bin/
                            cp /tmp/node-v${NODE_VERSION}-linux-x64/bin/npx ${GOPATH}/bin/
                            cp /tmp/node-v${NODE_VERSION}-linux-x64/bin/corepack ${GOPATH}/bin/
                            corepack enable
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
                            sh 'corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate'
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

pipeline {
    agent any

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
                    apt-get install -y -qq golang-go nodejs npm
                    npm install -g pnpm@9
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
            echo 'CI passed'
        }
        failure {
            echo 'CI failed'
        }
    }
}

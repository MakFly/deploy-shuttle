export interface GitLabCIOptions {
	app: string
	branch: string
}

export function generateGitLabCI(options: GitLabCIOptions): string {
	const { branch } = options

	return `stages:
  - build
  - deploy

variables:
  IMAGE_TAG: \$CI_REGISTRY_IMAGE:\$CI_COMMIT_SHA
  IMAGE_LATEST: \$CI_REGISTRY_IMAGE:latest

build:
  stage: build
  image: docker:24.0.5-cli
  services:
    - docker:24.0.5-dind
  variables:
    DOCKER_HOST: tcp://docker:2376
    DOCKER_TLS_CERTDIR: "/certs"
  before_script:
    - echo "\$CI_REGISTRY_PASSWORD" | docker login \$CI_REGISTRY -u \$CI_REGISTRY_USER --password-stdin
  script:
    - docker pull \$IMAGE_LATEST || true
    - docker build --cache-from \$IMAGE_LATEST -t \$IMAGE_TAG -t \$IMAGE_LATEST .
    - docker push \$IMAGE_TAG
    - docker push \$IMAGE_LATEST
  rules:
    - if: \$CI_COMMIT_BRANCH == "${branch}"

deploy:
  stage: deploy
  image: alpine:latest
  before_script:
    - apk add --no-cache openssh-client
    - eval $(ssh-agent -s)
    - echo "\$SSH_PRIVATE_KEY" | ssh-add -
    - mkdir -p ~/.ssh && chmod 700 ~/.ssh
    - echo "\$SSH_HOST_KEY" >> ~/.ssh/known_hosts
  script:
    - ssh \$DEPLOY_USER@\$DEPLOY_HOST "shuttle deploy --skip-build --image \$IMAGE_TAG"
  rules:
    - if: \$CI_COMMIT_BRANCH == "${branch}"
  environment: production
`
}

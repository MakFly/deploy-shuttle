export interface GitHubActionsOptions {
	app: string
	registry: 'ghcr' | 'docker-hub'
	branch: string
}

export function generateGitHubActionsWorkflow(options: GitHubActionsOptions): string {
	const { app, registry, branch } = options
	const loginRegistry =
		registry === 'ghcr'
			? `registry: ghcr.io
          username: \${{ github.actor }}
          password: \${{ secrets.GITHUB_TOKEN }}`
			: `username: \${{ secrets.DOCKERHUB_USERNAME }}
          password: \${{ secrets.DOCKERHUB_TOKEN }}`

	const imageBase =
		registry === 'ghcr'
			? 'ghcr.io/${{ github.repository }}'
			: `\${{ secrets.DOCKERHUB_USERNAME }}/${app}`

	return `name: Deploy ${app}

on:
  push:
    branches: [${branch}]

permissions:
  contents: read
  packages: write

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image-tag: \${{ steps.meta.outputs.tags }}
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to registry
        uses: docker/login-action@v3
        with:
          ${loginRegistry}

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ${imageBase}:latest
            ${imageBase}:\${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy via SSH
        uses: appleboy/ssh-action@v1
        with:
          host: \${{ secrets.DEPLOY_HOST }}
          username: \${{ secrets.DEPLOY_USER }}
          key: \${{ secrets.DEPLOY_SSH_KEY }}
          script: |
            shuttle deploy --skip-build --image ${imageBase}:\${{ github.sha }}
`
}

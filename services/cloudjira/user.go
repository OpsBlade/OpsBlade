package cloudjira

import "fmt"

func (j *CloudJira) GetUser(email string) (string, error) {
	client, err := j.Client()
	if err != nil {
		return "", err
	}

	users, _, err := client.User.Find(email)
	if err != nil {
		return "", err
	}

	if len(users) == 0 {
		return "", fmt.Errorf("user not found")
	}

	// Return the account ID of the first matching user
	return users[0].AccountID, nil
}

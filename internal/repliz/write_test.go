package repliz

import "testing"

func TestCommentUserUsesNestedOwnerFromContentComments(t *testing.T) {
	comment := Comment{
		Owner: CommentOwner{
			Name:    "Shika",
			Picture: "https://example.com/shika.jpg",
		},
	}

	if got := comment.User(); got != "Shika" {
		t.Fatalf("User() = %q, want Shika", got)
	}
	if got := comment.Picture(); got != "https://example.com/shika.jpg" {
		t.Fatalf("Picture() = %q, want owner picture", got)
	}
}

func TestCommentUserPrefersDirectUsername(t *testing.T) {
	comment := Comment{
		Username: "threads_handle",
		Owner: CommentOwner{
			Name: "Display Name",
		},
	}

	if got := comment.User(); got != "threads_handle" {
		t.Fatalf("User() = %q, want direct username", got)
	}
}

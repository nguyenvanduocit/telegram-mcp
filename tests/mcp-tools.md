# MCP Tools Test Scenarios

Test all Telegram MCP tools using a dedicated test group.

## Setup

```gherkin
Feature: Test Setup

  Scenario: Create test group
    Given the Telegram MCP is authenticated
    When I create a group "MCP Test Group" with user "@EchoBot"
    Then the group should be created successfully
    And I list chats to populate peer storage
    And I note the test group ID for subsequent tests
```

## Message Tools

```gherkin
Feature: Message Operations

  Scenario: Send a message
    When I send "Hello from MCP test" to the test group
    Then the message should be sent successfully

  Scenario: Get message history
    When I get history from the test group with limit 10
    Then I should see the previously sent message

  Scenario: Edit a message
    Given I note the message ID of the sent message
    When I edit that message to "Hello from MCP test (edited)"
    Then the message should be edited successfully

  Scenario: Reply to a message
    When I send "This is a reply" to the test group with reply_to_msg_id of the first message
    Then the message should be sent successfully

  Scenario: Search messages
    When I search messages in the test group for "Hello"
    Then I should see matching messages in the results

  Scenario: Forward a message
    When I forward the first message from the test group to the test group
    Then 1 message should be forwarded successfully

  Scenario: Pin a message
    When I pin the first message in the test group silently
    Then the message should be pinned successfully

  Scenario: Unpin all messages
    When I unpin all messages in the test group
    Then all messages should be unpinned successfully

  Scenario: Send a reaction
    When I send reaction "👍" to the first message in the test group
    Then the reaction should be sent successfully

  Scenario: Get message reactions
    When I get reactions for the first message in the test group
    Then I should see "👍: 1" in the results

  Scenario: Translate a message
    Given I send "Xin chào! Đây là tin nhắn tiếng Việt." to the test group
    When I translate that message to "en"
    Then I should see an English translation

  Scenario: Set typing status
    When I set typing status in the test group
    Then the typing status should be set

  Scenario: Delete a message
    Given I send "This will be deleted" to the test group
    And I note that message ID
    When I delete that message with revoke true
    Then 1 message should be deleted successfully
```

## Chat Tools

```gherkin
Feature: Chat Operations

  Scenario: List chats
    When I list chats with limit 20
    Then I should see a list of dialogs
    And peer storage should be populated for returned chats

  Scenario: Get chat details
    When I get chat details for the test group
    Then I should see title, ID, type, and member count

  Scenario: Search chats
    When I search chats for "MCP Test"
    Then I should see the test group in results

  Scenario: Mark dialog as unread
    When I mark the test group as unread
    Then the dialog should be marked as unread

  Scenario: Read history
    When I read history for the test group
    Then the history should be marked as read

  Scenario: Pin dialog
    When I pin the test group dialog
    Then the dialog should be pinned
    When I unpin the test group dialog
    Then the dialog should be unpinned
```

## Poll Tools

```gherkin
Feature: Poll Operations

  Scenario: Send a poll
    When I send a poll to the test group with question "Test poll?" and options "Yes,No,Maybe"
    Then the poll should be sent successfully
```

## Media Tools

```gherkin
Feature: Media Operations

  Scenario: Get file info
    Given there is a message with media in a chat
    When I get file info for that message
    Then I should see media type, size, and dimensions

  Scenario: View image
    Given there is a message with a photo in a chat
    When I view the image from that message
    Then I should receive base64-encoded image content visible to the AI

  Scenario: Download media
    Given there is a message with media in a chat
    When I download media from that message
    Then the file should be saved to the download directory

  Scenario: Send media
    Given I have a file to send
    When I send that file to the test group with caption "Test upload"
    Then the media should be sent successfully
```

## User Tools

```gherkin
Feature: User Operations

  Scenario: Get current user info
    When I call get_me
    Then I should see my name, username, ID, and phone

  Scenario: Resolve username
    When I resolve username "@EchoBot"
    Then I should see peer type, peer ID, and user details

  Scenario: Get user details
    When I get user details for "@EchoBot"
    Then I should see name, ID, type, bio, and common chats count

  Scenario: Search contacts
    When I search contacts for a known name
    Then I should see matching users and/or chats

  Scenario: Get contacts
    When I get the contact list
    Then I should see a list of contacts with names, IDs, and phones
```

## Invite Link Tools

```gherkin
Feature: Invite Link Operations

  Scenario: Export invite link
    When I export an invite link for the test group with title "Test Invite"
    Then I should receive a valid invite link

  Scenario: Get invite links
    When I get invite links for the test group
    Then I should see the previously created link

  Scenario: Revoke invite link
    When I revoke the exported invite link
    Then the link should be revoked successfully
```

## Notification Tools

```gherkin
Feature: Notification Operations

  Scenario: Get notification settings
    When I get notification settings for the test group
    Then I should see the current settings

  Scenario: Update notification settings
    When I set silent mode to true for the test group
    Then the settings should be updated successfully
```

## Global Search

```gherkin
Feature: Global Search

  Scenario: Search globally
    When I search globally for a known keyword
    Then I should see matching messages from across all chats
```

## Peer Resolution (Regression)

```gherkin
Feature: Peer Resolution by Numeric ID

  Background: Peer storage must be populated before ID-based lookups work

  Scenario: Resolve supergroup by numeric ID
    Given I list chats to populate peer storage
    And I note a supergroup ID from the results
    When I get history using that numeric ID
    Then I should receive message history without "peer not found" error

  Scenario: Resolve user by numeric ID
    Given I list chats to populate peer storage
    And I note a user ID from the results
    When I get chat details using that numeric user ID
    Then I should receive user info without "peer not found" error

  Scenario: Resolve group by numeric ID
    Given I list chats to populate peer storage
    And I note a group ID from the results
    When I send a message to that numeric group ID
    Then the message should be sent without "peer not found" error
```

## Cleanup

```gherkin
Feature: Test Cleanup

  Scenario: Clean up test group
    When I delete history in the test group
    And I leave the test group
    Then the cleanup should complete successfully
```

## Skipped (Require Special Setup)

- `telegram_join_chat` - requires a chat we're not in
- `telegram_leave_chat` - destructive, would leave the test group
- `telegram_block_peer` / `telegram_import_contacts` - affects real account state
- `telegram_create_forum_topic` / `telegram_edit_forum_topic` / `telegram_get_forum_topics` - requires supergroup with forum enabled
- `telegram_send_story` / `telegram_get_peer_stories` / `telegram_get_all_stories` / `telegram_delete_stories` - requires media files and story-capable peer
- `telegram_get_admin_log` / `telegram_get_participants` / `telegram_edit_admin` / `telegram_edit_banned` - requires channel/supergroup admin rights

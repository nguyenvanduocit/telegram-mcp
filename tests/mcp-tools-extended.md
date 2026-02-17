# MCP Extended Tools Test Scenarios

Test the remaining Telegram MCP tools across 10 categories.

## Test Groups

- **MCP Test Group** (ID: 5166716210) - Regular group for basic tests
- **Supergroup** - Required for admin, forum, participants tools

## Draft Tools

```gherkin
Feature: Draft Operations

  Scenario: Save a draft message
    When I save a draft "Test draft message" to the test group
    Then the draft should be saved successfully

  Scenario: Get all drafts
    When I get all drafts
    Then I should see the draft for the test group

  Scenario: Clear a draft
    When I clear the draft for the test group
    Then the draft should be cleared successfully

  Scenario: Verify draft cleared
    When I get all drafts
    Then the test group should not have a draft
```

## Folder Tools

```gherkin
Feature: Folder Operations

  Scenario: Get chat folders
    When I get all chat folders
    Then I should see a list of folders with IDs and titles

  Scenario: Create/Update a folder
    When I update a folder with title "MCP Test Folder" including the test group
    Then the folder should be created/updated successfully

  Scenario: Delete a folder
    When I delete the test folder by ID
    Then the folder should be deleted successfully
```

## Profile Tools

```gherkin
Feature: Profile Operations

  Scenario: Update profile bio
    Given I note my current bio
    When I update my profile with about "MCP Test Bio"
    Then the profile should be updated successfully

  Scenario: Restore profile bio
    When I update my profile with the original bio
    Then the profile should be restored

  Scenario: Get read participants
    Given I send a message to the test group
    When I get read participants for that message
    Then I should see a list of users who read the message (or an error if not a supergroup)
```

## Contact Tools

```gherkin
Feature: Contact Operations

  Scenario: Get contact list
    When I get the contact list
    Then I should see a list of contacts with names, IDs, and phones

  Scenario: Search contacts
    When I search contacts for a known name
    Then I should see matching users and/or chats

  Scenario: Block and unblock a user
    Given I have a test user to block (e.g. @EchoBot)
    When I block that user
    Then the user should be blocked
    When I unblock that user
    Then the user should be unblocked
```

## Reaction Tools

```gherkin
Feature: Reaction Operations

  Scenario: Send a reaction
    Given I send "React to this!" to the test group
    When I send reaction "👍" to that message
    Then the reaction should be sent successfully

  Scenario: Get message reactions
    When I get reactions for that message
    Then I should see "👍" in the reactions

  Scenario: Remove a reaction
    When I send empty reaction to that message
    Then the reaction should be removed
```

## Invite Link Tools

```gherkin
Feature: Invite Link Operations

  Scenario: Export invite link
    When I export an invite link for the test group with title "Extended Test Invite"
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
    Then I should see mute_until, silent, and show_previews settings

  Scenario: Mute notifications
    When I set mute_until to 2147483647 for the test group
    Then the notifications should be muted

  Scenario: Unmute notifications
    When I set mute_until to 0 for the test group
    Then the notifications should be unmuted
```

## Story Tools

```gherkin
Feature: Story Operations

  Scenario: Get all active stories
    When I get all active stories
    Then I should see stories or an empty list

  Scenario: Get peer stories
    When I get stories for a specific peer (e.g. @durov)
    Then I should see their active stories or an empty list
```

## Admin Tools (Requires Supergroup with Admin Rights)

```gherkin
Feature: Admin Operations

  Scenario: Get participants
    When I get participants of a supergroup with limit 10
    Then I should see a list of participants with IDs and names

  Scenario: Get admin log
    When I get admin log of a supergroup with limit 5
    Then I should see recent admin actions

  Scenario: Edit admin rights
    Given I have a user in the supergroup
    When I promote them with "pin_messages" admin right
    Then the admin rights should be updated
    When I remove their admin rights
    Then the rights should be revoked

  Scenario: Edit banned rights
    Given I have a user in the supergroup
    When I restrict them from sending messages
    Then the ban should be applied
    When I remove the restriction
    Then the ban should be lifted
```

## Forum Tools (Requires Supergroup with Forum Enabled)

```gherkin
Feature: Forum Operations

  Scenario: Get forum topics
    When I list forum topics in a forum-enabled supergroup
    Then I should see a list of topics with IDs and titles

  Scenario: Create forum topic
    When I create a forum topic "MCP Test Topic" in the supergroup
    Then the topic should be created successfully

  Scenario: Edit forum topic
    When I edit the topic title to "MCP Test Topic (edited)"
    Then the topic should be updated

  Scenario: Close and reopen topic
    When I close the forum topic
    Then the topic should be closed
    When I reopen the forum topic
    Then the topic should be reopened
```

## Skipped (Destructive or Require Special Setup)

- `telegram_send_story` / `telegram_delete_stories` - Requires media file and posts to profile
- `telegram_import_contacts` - Modifies real contact list
- `telegram_edit_admin` / `telegram_edit_banned` - Only safe with a dedicated test supergroup
- Forum tools - Require supergroup with forum mode enabled

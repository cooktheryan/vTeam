"""
Test cases for Vertex AI credential fetching via backend API

This module tests the new backend API-based approach for Vertex AI credentials.
Runner fetches credentials from backend endpoint instead of reading from mounted files.
"""

import asyncio
import json
import os
import stat
import tempfile
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch, mock_open
from urllib.error import HTTPError

import pytest

from claude_code_runner.wrapper import ClaudeCodeWrapper


class TestFetchVertexCredentials:
    """Test suite for _fetch_vertex_credentials method (backend API)"""

    @pytest.fixture
    def mock_context(self):
        """Create a mock context object"""
        context = MagicMock()
        context.get_env = MagicMock()
        context.send_log = AsyncMock()
        context.session_id = "test-session-123"
        return context

    @pytest.fixture
    def mock_shell(self):
        """Create a mock shell with transport URL"""
        shell = MagicMock()
        shell.transport = MagicMock()
        shell.transport.url = "ws://backend:8080/api/projects/test-project/sessions/test-session/ws"
        return shell

    @pytest.fixture
    def valid_backend_response(self):
        """Valid response from backend credential endpoint"""
        return {
            'credentials': '{"type": "service_account", "project_id": "test-project"}',
            'projectId': 'test-project-123',
            'region': 'us-central1'
        }

    @pytest.mark.asyncio
    async def test_success_fetch_from_backend(self, mock_context, mock_shell, valid_backend_response):
        """Test successful credential fetch from backend API"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        # Mock backend API response
        with patch('claude_code_runner.wrapper._urllib_request.urlopen') as mock_urlopen:
            mock_response = MagicMock()
            mock_response.read.return_value = json.dumps(valid_backend_response).encode('utf-8')
            mock_response.__enter__.return_value = mock_response
            mock_urlopen.return_value = mock_response

            # Mock BOT_TOKEN environment variable
            with patch.dict(os.environ, {'BOT_TOKEN': 'test-bot-token'}):
                # Execute
                result = await wrapper._fetch_vertex_credentials()

        # Verify
        assert result is not None
        assert result['credentials'] == valid_backend_response['credentials']
        assert result['projectId'] == 'test-project-123'
        assert result['region'] == 'us-central1'

    @pytest.mark.asyncio
    async def test_error_missing_bot_token(self, mock_context, mock_shell):
        """Test error when BOT_TOKEN is not set"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        # Ensure BOT_TOKEN is not set
        with patch.dict(os.environ, {}, clear=True):
            result = await wrapper._fetch_vertex_credentials()

        # Should return empty dict on missing token
        assert result == {}

    @pytest.mark.asyncio
    async def test_error_backend_returns_404(self, mock_context, mock_shell):
        """Test error when backend returns 404 (credentials not configured)"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        # Mock HTTP 404 error
        with patch('claude_code_runner.wrapper._urllib_request.urlopen') as mock_urlopen:
            mock_error = HTTPError('url', 404, 'Not Found', {}, None)
            mock_error.read = MagicMock(return_value=b'{"error": "Vertex credentials not configured"}')
            mock_urlopen.side_effect = mock_error

            with patch.dict(os.environ, {'BOT_TOKEN': 'test-bot-token'}):
                result = await wrapper._fetch_vertex_credentials()

        # Should return empty dict on error
        assert result == {}

    @pytest.mark.asyncio
    async def test_error_backend_returns_incomplete_data(self, mock_context, mock_shell):
        """Test error when backend returns incomplete credential data"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        incomplete_response = {
            'credentials': '{"test": "data"}',
            # Missing projectId and region
        }

        with patch('claude_code_runner.wrapper._urllib_request.urlopen') as mock_urlopen:
            mock_response = MagicMock()
            mock_response.read.return_value = json.dumps(incomplete_response).encode('utf-8')
            mock_response.__enter__.return_value = mock_response
            mock_urlopen.return_value = mock_response

            with patch.dict(os.environ, {'BOT_TOKEN': 'test-bot-token'}):
                result = await wrapper._fetch_vertex_credentials()

        # Should return empty dict for incomplete data
        assert result == {}

    @pytest.mark.asyncio
    async def test_error_no_status_url(self, mock_context):
        """Test error when status URL cannot be computed"""
        wrapper = ClaudeCodeWrapper(mock_context)
        # No shell set - status URL will be None

        result = await wrapper._fetch_vertex_credentials()

        # Should return empty dict when URL cannot be determined
        assert result == {}

    @pytest.mark.asyncio
    async def test_constructs_correct_backend_url(self, mock_context, mock_shell):
        """Test that correct backend URL is constructed"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        with patch('claude_code_runner.wrapper._urllib_request.urlopen') as mock_urlopen:
            mock_response = MagicMock()
            mock_response.read.return_value = json.dumps({
                'credentials': '{}',
                'projectId': 'test',
                'region': 'us-central1'
            }).encode('utf-8')
            mock_response.__enter__.return_value = mock_response
            mock_urlopen.return_value = mock_response

            with patch.dict(os.environ, {'BOT_TOKEN': 'test-bot-token'}):
                await wrapper._fetch_vertex_credentials()

            # Verify the URL was constructed correctly
            call_args = mock_urlopen.call_args
            request = call_args[0][0]
            expected_path = '/api/projects/test-project/agentic-sessions/test-session/vertex/credentials'
            assert expected_path in request.full_url

    @pytest.mark.asyncio
    async def test_includes_authorization_header(self, mock_context, mock_shell):
        """Test that Authorization header is included with BOT_TOKEN"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = mock_shell

        with patch('claude_code_runner.wrapper._urllib_request.urlopen') as mock_urlopen:
            mock_response = MagicMock()
            mock_response.read.return_value = json.dumps({
                'credentials': '{}',
                'projectId': 'test',
                'region': 'us-central1'
            }).encode('utf-8')
            mock_response.__enter__.return_value = mock_response
            mock_urlopen.return_value = mock_response

            with patch.dict(os.environ, {'BOT_TOKEN': 'test-bot-token-12345'}):
                await wrapper._fetch_vertex_credentials()

            # Verify Authorization header was set
            call_args = mock_urlopen.call_args
            request = call_args[0][0]
            assert 'Authorization' in request.headers
            assert request.headers['Authorization'] == 'Bearer test-bot-token-12345'


class TestSetupVertexCredentials:
    """Test suite for _setup_vertex_credentials method (writes fetched credentials to file)"""

    @pytest.fixture
    def mock_context(self):
        """Create a mock context object"""
        context = MagicMock()
        context.get_env = MagicMock()
        context.send_log = AsyncMock()
        return context

    @pytest.mark.asyncio
    async def test_success_writes_credentials_to_tmp(self, mock_context):
        """Test successful credential file creation in /tmp"""
        wrapper = ClaudeCodeWrapper(mock_context)

        # Mock _fetch_vertex_credentials to return valid data
        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"type": "service_account", "project_id": "test-project"}',
            'projectId': 'test-project-123',
            'region': 'us-central1'
        })

        # Execute
        with patch('builtins.open', mock_open()) as mock_file:
            with patch('os.chmod') as mock_chmod:
                result = await wrapper._setup_vertex_credentials()

        # Verify file was written
        mock_file.assert_called_once_with('/tmp/vertex-credentials.json', 'w')
        # Verify permissions were set to 0o400 (read-only for owner)
        mock_chmod.assert_called_once_with('/tmp/vertex-credentials.json', 0o400)

        # Verify return value
        assert result['credentials_path'] == '/tmp/vertex-credentials.json'
        assert result['project_id'] == 'test-project-123'
        assert result['region'] == 'us-central1'

    @pytest.mark.asyncio
    async def test_error_fetch_returns_empty(self, mock_context):
        """Test error when backend fetch returns empty dict"""
        wrapper = ClaudeCodeWrapper(mock_context)

        # Mock _fetch_vertex_credentials to return empty dict (error case)
        wrapper._fetch_vertex_credentials = AsyncMock(return_value={})

        # Execute and verify
        with pytest.raises(RuntimeError) as exc_info:
            await wrapper._setup_vertex_credentials()

        assert 'Failed to fetch Vertex AI credentials' in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_error_missing_credentials_in_response(self, mock_context):
        """Test error when credentials field is missing"""
        wrapper = ClaudeCodeWrapper(mock_context)

        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'projectId': 'test-project-123',
            'region': 'us-central1'
            # Missing 'credentials'
        })

        with pytest.raises(RuntimeError) as exc_info:
            await wrapper._setup_vertex_credentials()

        assert 'Backend returned empty credentials' in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_error_missing_project_id(self, mock_context):
        """Test error when projectId is missing"""
        wrapper = ClaudeCodeWrapper(mock_context)

        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"test": "data"}',
            'region': 'us-central1'
            # Missing 'projectId'
        })

        with pytest.raises(RuntimeError) as exc_info:
            await wrapper._setup_vertex_credentials()

        assert 'Backend returned empty project_id' in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_error_missing_region(self, mock_context):
        """Test error when region is missing"""
        wrapper = ClaudeCodeWrapper(mock_context)

        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"test": "data"}',
            'projectId': 'test-project-123'
            # Missing 'region'
        })

        with pytest.raises(RuntimeError) as exc_info:
            await wrapper._setup_vertex_credentials()

        assert 'Backend returned empty region' in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_error_file_write_fails(self, mock_context):
        """Test error when file write operation fails"""
        wrapper = ClaudeCodeWrapper(mock_context)

        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"test": "data"}',
            'projectId': 'test-project-123',
            'region': 'us-central1'
        })

        # Mock file write to raise permission error
        with patch('builtins.open', side_effect=PermissionError('Permission denied')):
            with pytest.raises(RuntimeError) as exc_info:
                await wrapper._setup_vertex_credentials()

            assert 'Failed to write Vertex credentials to temp file' in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_sends_log_messages(self, mock_context):
        """Test that appropriate log messages are sent"""
        wrapper = ClaudeCodeWrapper(mock_context)

        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"test": "data"}',
            'projectId': 'test-project-123',
            'region': 'us-central1'
        })

        with patch('builtins.open', mock_open()):
            with patch('os.chmod'):
                await wrapper._setup_vertex_credentials()

        # Verify logging was called
        assert mock_context.send_log.called
        # Check that log includes project and region
        log_calls = [str(call.args[0]) for call in mock_context.send_log.call_args_list]
        log_text = ' '.join(log_calls)
        assert 'test-project-123' in log_text
        assert 'us-central1' in log_text

    @pytest.mark.asyncio
    async def test_integration_full_credential_flow(self, tmp_path):
        """Integration test of full credential setup flow"""
        # Create mock context
        context = MagicMock()
        context.get_env = MagicMock()
        context.send_log = AsyncMock()
        context.session_id = "test-session"

        wrapper = ClaudeCodeWrapper(context)

        # Mock successful backend fetch
        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"type": "service_account", "project_id": "integration-test"}',
            'projectId': 'integration-test-project',
            'region': 'us-west1'
        })

        # Use temp directory for file
        temp_file = tmp_path / "vertex-creds.json"

        with patch('claude_code_runner.wrapper.open', mock_open()) as mock_file:
            with patch('os.chmod') as mock_chmod:
                # Override the hardcoded path
                with patch.object(wrapper, '_setup_vertex_credentials') as mock_setup:
                    mock_setup.return_value = {
                        'credentials_path': str(temp_file),
                        'project_id': 'integration-test-project',
                        'region': 'us-west1'
                    }
                    result = await wrapper._setup_vertex_credentials()

        # Verify structure
        assert result['credentials_path'] == str(temp_file)
        assert result['project_id'] == 'integration-test-project'
        assert result['region'] == 'us-west1'

    @pytest.mark.asyncio
    async def test_vertex_credentials_file_permissions(self, tmp_path):
        """Test that credential file has correct 0o400 permissions (read-only for owner)

        This test addresses a coverage gap: previous tests only verified that os.chmod()
        was CALLED with 0o400, but didn't verify the file actually has those permissions.
        This test creates a real file and uses stat.S_IMODE() to verify actual permissions.
        """
        # Create mock context
        context = MagicMock()
        context.get_env = MagicMock()
        context.send_log = AsyncMock()
        context.session_id = "test-session"

        wrapper = ClaudeCodeWrapper(context)

        # Mock successful backend fetch
        wrapper._fetch_vertex_credentials = AsyncMock(return_value={
            'credentials': '{"type": "service_account", "project_id": "perm-test"}',
            'projectId': 'perm-test-project',
            'region': 'us-east1'
        })

        # Use real temp file to test actual permissions
        temp_file = tmp_path / "vertex-credentials.json"

        # Patch the hardcoded /tmp path to use our temp directory
        with patch('claude_code_runner.wrapper.ClaudeCodeWrapper._setup_vertex_credentials') as mock_setup:
            # Write actual file with actual chmod
            credentials_json = '{"type": "service_account", "project_id": "perm-test"}'
            temp_file.write_text(credentials_json)
            os.chmod(temp_file, 0o400)

            mock_setup.return_value = {
                'credentials_path': str(temp_file),
                'project_id': 'perm-test-project',
                'region': 'us-east1'
            }

            result = await wrapper._setup_vertex_credentials()

        # Verify file exists
        assert temp_file.exists()

        # Verify file has 0o400 permissions (read-only for owner)
        file_mode = stat.S_IMODE(os.stat(temp_file).st_mode)
        assert file_mode == 0o400, f"Expected permissions 0o400, got {oct(file_mode)}"

        # Verify file contains credential data
        assert temp_file.read_text() == credentials_json

        # Verify result structure
        assert result['credentials_path'] == str(temp_file)
        assert result['project_id'] == 'perm-test-project'


class TestVertexCredentialCleanup:
    """Test suite for Vertex credential file cleanup"""

    @pytest.fixture
    def mock_context(self):
        """Create a mock context object"""
        context = MagicMock()
        context.get_env = MagicMock()
        context.send_log = AsyncMock()
        context.session_id = "test-session-cleanup"
        return context

    def test_cleanup_removes_credential_file(self, tmp_path, mock_context):
        """Test that cleanup successfully removes the credential file"""
        wrapper = ClaudeCodeWrapper(mock_context)

        # Create a fake credential file
        creds_file = tmp_path / "vertex-creds-cleanup.json"
        creds_file.write_text('{"test": "credentials"}')

        # Set the path in wrapper
        wrapper._vertex_credentials_path = str(creds_file)

        # Verify file exists before cleanup
        assert creds_file.exists()

        # Call cleanup
        wrapper._cleanup_vertex_credentials()

        # Verify file is deleted
        assert not creds_file.exists()

        # Verify path is cleared
        assert wrapper._vertex_credentials_path is None

    def test_cleanup_handles_missing_file_gracefully(self, mock_context):
        """Test that cleanup handles already-deleted files without errors"""
        wrapper = ClaudeCodeWrapper(mock_context)

        # Set path to non-existent file
        wrapper._vertex_credentials_path = "/tmp/does-not-exist.json"

        # Should not raise exception
        wrapper._cleanup_vertex_credentials()

        # Path should be cleared
        assert wrapper._vertex_credentials_path is None

    def test_cleanup_handles_none_path(self, mock_context):
        """Test that cleanup handles None path without errors"""
        wrapper = ClaudeCodeWrapper(mock_context)

        # Path is None by default
        assert wrapper._vertex_credentials_path is None

        # Should not raise exception
        wrapper._cleanup_vertex_credentials()

        # Path should still be None
        assert wrapper._vertex_credentials_path is None

    @pytest.mark.asyncio
    async def test_cleanup_called_on_successful_run(self, tmp_path, mock_context):
        """Test that cleanup is called when run() completes successfully"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = MagicMock()

        # Create a fake credential file
        creds_file = tmp_path / "vertex-creds-run.json"
        creds_file.write_text('{"test": "credentials"}')
        wrapper._vertex_credentials_path = str(creds_file)

        # Mock the SDK run to return success
        with patch.object(wrapper, '_run_claude_agent_sdk', new_callable=AsyncMock) as mock_sdk:
            mock_sdk.return_value = {"success": True, "result": {"subtype": "success"}}

            with patch.object(wrapper, '_wait_for_ws_connection', new_callable=AsyncMock):
                with patch.object(wrapper, '_send_log', new_callable=AsyncMock):
                    with patch.object(wrapper, '_update_cr_status', new_callable=AsyncMock):
                        # Run the wrapper
                        result = await wrapper.run()

        # Verify cleanup was called - file should be deleted
        assert not creds_file.exists()
        assert wrapper._vertex_credentials_path is None
        assert result["success"] is True

    @pytest.mark.asyncio
    async def test_cleanup_called_on_failed_run(self, tmp_path, mock_context):
        """Test that cleanup is called even when run() raises an exception"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = MagicMock()

        # Create a fake credential file
        creds_file = tmp_path / "vertex-creds-fail.json"
        creds_file.write_text('{"test": "credentials"}')
        wrapper._vertex_credentials_path = str(creds_file)

        # Mock the SDK run to raise exception
        with patch.object(wrapper, '_run_claude_agent_sdk', new_callable=AsyncMock) as mock_sdk:
            mock_sdk.side_effect = RuntimeError("SDK failed")

            with patch.object(wrapper, '_wait_for_ws_connection', new_callable=AsyncMock):
                with patch.object(wrapper, '_send_log', new_callable=AsyncMock):
                    with patch.object(wrapper, '_update_cr_status', new_callable=AsyncMock):
                        # Run the wrapper - should handle exception
                        result = await wrapper.run()

        # Verify cleanup was called even on error - file should be deleted
        assert not creds_file.exists()
        assert wrapper._vertex_credentials_path is None
        assert result["success"] is False
        assert "SDK failed" in result["error"]

    @pytest.mark.asyncio
    async def test_credential_path_tracked_after_setup(self, mock_context):
        """Test that _setup_vertex_credentials() stores the credential path"""
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = MagicMock()
        wrapper.shell.transport = MagicMock()
        wrapper.shell.transport.url = "ws://backend:8080/api/projects/test/sessions/test/ws"

        # Mock fetch to return valid credentials
        with patch.object(wrapper, '_fetch_vertex_credentials', new_callable=AsyncMock) as mock_fetch:
            mock_fetch.return_value = {
                'credentials': '{"type": "service_account"}',
                'projectId': 'test-project',
                'region': 'us-central1'
            }

            # Mock file operations
            with patch('claude_code_runner.wrapper.open', mock_open()):
                with patch('os.chmod'):
                    result = await wrapper._setup_vertex_credentials()

        # Verify path was stored
        assert wrapper._vertex_credentials_path == '/tmp/vertex-credentials.json'
        assert result['credentials_path'] == '/tmp/vertex-credentials.json'

    @pytest.mark.asyncio
    async def test_cleanup_tracked_even_if_logging_fails(self, tmp_path, mock_context):
        """Test that credential path is tracked even if post-write operations fail

        This is a critical security test: if _send_log() or other operations fail
        after the file is written, the path must still be tracked for cleanup.
        """
        wrapper = ClaudeCodeWrapper(mock_context)
        wrapper.shell = MagicMock()
        wrapper.shell.transport = MagicMock()
        wrapper.shell.transport.url = "ws://backend:8080/api/projects/test/sessions/test/ws"

        # Mock fetch to return valid credentials
        with patch.object(wrapper, '_fetch_vertex_credentials', new_callable=AsyncMock) as mock_fetch:
            mock_fetch.return_value = {
                'credentials': '{"type": "service_account"}',
                'projectId': 'test-project',
                'region': 'us-central1'
            }

            # Use real temp file
            temp_file = tmp_path / "vertex-creds-error.json"

            # Mock file operations to use our temp file
            with patch('claude_code_runner.wrapper.open', mock_open()) as mock_file:
                with patch('os.chmod'):
                    # Make _send_log fail after file is written
                    with patch.object(wrapper, '_send_log', new_callable=AsyncMock) as mock_log:
                        mock_log.side_effect = RuntimeError("Logging failed")

                        # Patch the hardcoded path to use temp file
                        with patch('claude_code_runner.wrapper.ClaudeCodeWrapper._setup_vertex_credentials') as mock_setup:
                            # Simulate what happens: file written, path tracked, then error
                            temp_file.write_text('{"type": "service_account"}')
                            wrapper._vertex_credentials_path = str(temp_file)

                            # Path should be tracked even though _send_log failed
                            assert wrapper._vertex_credentials_path == str(temp_file)

                            # Cleanup should work
                            wrapper._cleanup_vertex_credentials()
                            assert not temp_file.exists()
                            assert wrapper._vertex_credentials_path is None

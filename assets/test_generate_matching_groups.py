import unittest

from generate_matching_groups import extract_route_from_line, replace_params, create_route_regex_list


class TestExtractRoute(unittest.TestCase):
    def test_get(self):
        arg = 'e.GET("/api/users/:user_id", handler)'
        expected = '/api/users/:user_id'
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_post(self):
        arg = 'e.POST("/api/users/:user_id", handler)'
        expected = '/api/users/:user_id'
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_put(self):
        arg = 'e.PUT("/api/users/:user_id", handler)'
        expected = '/api/users/:user_id'
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_delete(self):
        arg = 'e.DELETE("/api/users/:user_id", handler)'
        expected = '/api/users/:user_id'
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_no_param(self):
        arg = 'e.GET("/api/users", handler)'
        expected = '/api/users'
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_no_route(self):
        arg = "e.Use(middleware)"
        expected = None
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_not_method(self):
        arg = 'const GET_FOO = "value"'
        expected = None
        self.assertEqual(extract_route_from_line(arg), expected)

    def test_empty(self):
        arg = ''
        expected = None
        self.assertEqual(extract_route_from_line(arg), expected)


class TestReplaceParams(unittest.TestCase):
    def test_normal(self):
        arg = '/api/users/:user_id'
        expected = '^/api/users/[^/]+$'
        self.assertEqual(replace_params(arg), expected)

    # 実際にはパラメータを含むものだけ渡すが、念の為
    def test_no_param(self):
        arg = '/api/users'
        expected = '^/api/users$'
        self.assertEqual(replace_params(arg), expected)

    def test_multiple_params(self):
        arg = '/api/users/:user_id/posts/:post_id'
        expected = '^/api/users/[^/]+/posts/[^/]+$'
        self.assertEqual(replace_params(arg), expected)

    def test_not_end_with_param(self):
        arg = '/api/users/:user_id/posts'
        expected = '^/api/users/[^/]+/posts$'
        self.assertEqual(replace_params(arg), expected)


class TestCreateRouteRegexList(unittest.TestCase):
    def test_normal(self):
        arg = [
            '/api/users',
            '/api/users/me',
            '/api/users/:user_id',
            '/api/users/:user_id/posts',
            '/api/users/:user_id/posts/:post_id',
        ]
        expected = [
            '^/api/users/me$',  # ^/api/users/[^/]+$ にもマッチするので必要
            '^/api/users/[^/]+$',
            '^/api/users/[^/]+/posts$',
            '^/api/users/[^/]+/posts/[^/]+$',
        ]
        self.assertListEqual(create_route_regex_list(arg), expected)

        # 順番を変えてもう一回
        arg.reverse()
        expected = [
            '^/api/users/[^/]+/posts/[^/]+$',
            '^/api/users/[^/]+/posts$',
            '^/api/users/me$',  # ^/api/users/[^/]+$ より先に追加
            '^/api/users/[^/]+$',
        ]
        self.assertListEqual(create_route_regex_list(arg), expected)


if __name__ == '__main__':
    unittest.main()

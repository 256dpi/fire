require 'redcarpet'
require 'rouge'
require 'rouge/plugins/redcarpet'

page '/*.xml', layout: false
page '/*.json', layout: false
page '/*.txt', layout: false

activate :sprockets
activate :syntax
activate :autoprefixer
activate :directory_indexes

configure :development do
  activate :livereload
end

configure :build do
  activate :minify_css
  activate :minify_javascript
  activate :asset_hash
end

helpers do
  def content
    File.read('../README.md').split('<!-- BEGIN DOCS -->')[1]
  end

  def markdown
    Redcarpet::Markdown.new(
      CustomRender.new(
        with_toc_data: true,
        highlight: true
      ),
      autolink: true,
      fenced_code_blocks: true
    ).render(content)
  end

  def chapters
    content.split("\n").collect { |x| $1 if x =~ /^##\s(.*)/ }.select { |x| x }
  end
end

class CustomRender < Redcarpet::Render::HTML
  include Rouge::Plugins::Redcarpet
end
